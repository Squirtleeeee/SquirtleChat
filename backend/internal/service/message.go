package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"squirtlechat/internal/model"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/idgen"
	pkgkafka "squirtlechat/pkg/kafka"

	"github.com/segmentio/kafka-go"
)

type MessageService struct {
	msgs         *store.MessageStore
	friend       *store.FriendStore
	idgen        *idgen.Generator
	writer       *kafka.Writer
	instanceID   string
	onSent       func(ctx context.Context, evt *store.SentEvent)
	onRecalled   func(ctx context.Context, evt *store.RecallEvent)
	onTyping     func(ctx context.Context, evt *TypingEvent)
	onReaction   func(ctx context.Context, evt *ReactionEvent)
	onPin        func(ctx context.Context, evt *PinEvent)
	onEdited     func(ctx context.Context, evt *EditEvent)
	onPollVote   func(ctx context.Context, evt *PollVoteEvent)
	onReminder   func(ctx context.Context, evt *ReminderEvent)
}

type ReminderEvent struct {
	Reminder *store.MessageReminder
}

type TypingEvent struct {
	ConversationID string
	FromUserID     int64
	Typing         bool
	ToUserIDs      []int64
}

type ReactionEvent struct {
	ConversationID string
	MsgID          int64
	UserID         int64
	Emoji          string
	Added          bool
	ToUserIDs      []int64
	Summaries      []store.ReactionSummary
}

type PinEvent struct {
	ConversationID string
	MsgID          int64
	UserID         int64
	Pinned         bool
	ToUserIDs      []int64
	Pins           []store.PinnedMessage
}

type EditEvent struct {
	Message   *model.Message
	ToUserIDs []int64
}

type PollVoteEvent struct {
	ConversationID string
	MsgID          int64
	UserID         int64
	OptionID       string
	ToUserIDs      []int64
	Result         *store.PollResult
}

func NewMessageService(msgs *store.MessageStore, friend *store.FriendStore, gen *idgen.Generator, writer *kafka.Writer, instanceID string) *MessageService {
	return &MessageService{msgs: msgs, friend: friend, idgen: gen, writer: writer, instanceID: instanceID}
}

func (s *MessageService) SetOnSent(fn func(ctx context.Context, evt *store.SentEvent)) {
	s.onSent = fn
}

func (s *MessageService) SetOnRecalled(fn func(ctx context.Context, evt *store.RecallEvent)) {
	s.onRecalled = fn
}

func (s *MessageService) SetOnTyping(fn func(ctx context.Context, evt *TypingEvent)) {
	s.onTyping = fn
}

func (s *MessageService) SetOnReaction(fn func(ctx context.Context, evt *ReactionEvent)) {
	s.onReaction = fn
}

func (s *MessageService) SetOnPin(fn func(ctx context.Context, evt *PinEvent)) {
	s.onPin = fn
}

func (s *MessageService) SetOnEdited(fn func(ctx context.Context, evt *EditEvent)) {
	s.onEdited = fn
}

func (s *MessageService) SetOnPollVote(fn func(ctx context.Context, evt *PollVoteEvent)) {
	s.onPollVote = fn
}

func (s *MessageService) SetOnReminder(fn func(ctx context.Context, evt *ReminderEvent)) {
	s.onReminder = fn
}

type sendReq struct {
	ClientMsgID      string `json:"client_msg_id"`
	ConversationID   string `json:"conversation_id"`
	ConversationType int8   `json:"conversation_type"`
	ToUserID         string `json:"to_user_id"`
	MsgType          int8   `json:"msg_type"`
	Content          string `json:"content"`
}

type ackPayload struct {
	ClientMsgID string `json:"client_msg_id"`
	MsgID       int64  `json:"msg_id"`
	Seq         int64  `json:"seq"`
	Status      string `json:"status"`
}

func (s *MessageService) HandleSend(ctx context.Context, fromUserID int64, deviceID string, raw json.RawMessage) (json.RawMessage, error) {
	var req sendReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if req.ClientMsgID == "" || req.Content == "" {
		return nil, errors.New("消息内容无效")
	}
	convID := req.ConversationID
	var toUserID int64
	if req.ToUserID != "" {
		var err error
		toUserID, err = strconv.ParseInt(req.ToUserID, 10, 64)
		if err != nil || toUserID <= 0 {
			return nil, errors.New("请指定有效的聊天对象")
		}
	}
	if convID == "" && req.ConversationType == model.ConvTypeDirect {
		if toUserID == 0 {
			return nil, errors.New("请指定聊天对象")
		}
		ok, err := s.friend.AreFriends(ctx, fromUserID, toUserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("你们还不是好友，请先添加好友")
		}
		convID = idgen.DirectConversationID(fromUserID, toUserID)
		_ = s.msgs.EnsureConversation(ctx, convID, model.ConvTypeDirect, []int64{fromUserID, toUserID})
	}
	if convID == "" {
		return nil, errors.New("会话不存在")
	}
	if err := s.ensureMember(ctx, convID, fromUserID); err != nil {
		return nil, err
	}
	if err := s.ensureCanPost(ctx, convID, fromUserID); err != nil {
		return nil, err
	}

	if existing, err := s.msgs.GetByClientMsgID(ctx, convID, req.ClientMsgID); err == nil && existing != nil {
		return json.Marshal(ackPayload{ClientMsgID: req.ClientMsgID, MsgID: existing.ID, Seq: existing.Seq, Status: "ok"})
	}

	seq, err := s.msgs.NextSeq(ctx, convID)
	if err != nil {
		return nil, err
	}
	msg := &model.Message{
		ID:             s.idgen.Next(),
		ConversationID: convID,
		FromUserID:     fromUserID,
		Seq:            seq,
		MsgType:        req.MsgType,
		Content:        req.Content,
		ClientMsgID:    req.ClientMsgID,
	}
	if msg.MsgType == 0 {
		msg.MsgType = model.MsgTypeText
	}
	if msg.MsgType == model.MsgTypePoll {
		if err := validatePollContent(msg.Content); err != nil {
			return nil, err
		}
	}
	if err := s.msgs.Insert(ctx, msg); err != nil {
		return nil, err
	}
	if msg.MsgType == model.MsgTypeText {
		_ = s.msgs.ReplaceHashtags(ctx, convID, msg.ID, extractHashtags(msg.Content))
	}
	_ = s.msgs.BumpLastReadSeq(ctx, convID, fromUserID, seq)

	toUsers, err := s.resolveRecipients(ctx, convID, fromUserID, toUserID)
	if err != nil {
		return nil, err
	}

	evt := store.SentEvent{
		MsgID:          msg.ID,
		ConversationID: convID,
		FromUserID:     fromUserID,
		ToUserIDs:      toUsers,
		Seq:            seq,
		MsgType:        msg.MsgType,
		Content:        msg.Content,
		ClientMsgID:    req.ClientMsgID,
		ExceptDevice:   deviceID,
		OriginInstance: s.instanceID,
	}
	if s.onSent != nil {
		s.onSent(ctx, &evt)
	}
	if err := pkgkafka.Publish(ctx, s.writer, []byte(convID), evt.Bytes()); err != nil {
		log.Printf("kafka publish: %v", err)
	}

	return json.Marshal(ackPayload{ClientMsgID: req.ClientMsgID, MsgID: msg.ID, Seq: seq, Status: "ok"})
}

// InjectAgentReply inserts a bot message and dispatches it like a normal send.
func (s *MessageService) InjectAgentReply(ctx context.Context, botUserID, peerUserID int64, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("empty agent reply")
	}
	convID := idgen.DirectConversationID(botUserID, peerUserID)
	_ = s.msgs.EnsureConversation(ctx, convID, model.ConvTypeDirect, []int64{botUserID, peerUserID})

	clientMsgID := fmt.Sprintf("agent-%d", s.idgen.Next())
	seq, err := s.msgs.NextSeq(ctx, convID)
	if err != nil {
		return err
	}
	msg := &model.Message{
		ID:             s.idgen.Next(),
		ConversationID: convID,
		FromUserID:     botUserID,
		Seq:            seq,
		MsgType:        model.MsgTypeText,
		Content:        content,
		ClientMsgID:    clientMsgID,
	}
	if err := s.msgs.Insert(ctx, msg); err != nil {
		return err
	}
	evt := store.SentEvent{
		MsgID:          msg.ID,
		ConversationID: convID,
		FromUserID:     botUserID,
		ToUserIDs:      []int64{peerUserID},
		Seq:            seq,
		MsgType:        msg.MsgType,
		Content:        msg.Content,
		ClientMsgID:    clientMsgID,
		OriginInstance: s.instanceID,
	}
	if s.onSent != nil {
		s.onSent(ctx, &evt)
	}
	if err := pkgkafka.Publish(ctx, s.writer, []byte(convID), evt.Bytes()); err != nil {
		log.Printf("kafka publish: %v", err)
	}
	return nil
}

func (s *MessageService) PostSystemMessage(ctx context.Context, convID, content string) error {
	content = strings.TrimSpace(content)
	if content == "" || convID == "" {
		return nil
	}
	if utf8.RuneCountInString(content) > 500 {
		content = string([]rune(content)[:500])
	}
	clientMsgID := fmt.Sprintf("sys-%d", s.idgen.Next())
	seq, err := s.msgs.NextSeq(ctx, convID)
	if err != nil {
		return err
	}
	msg := &model.Message{
		ID:             s.idgen.Next(),
		ConversationID: convID,
		FromUserID:     0,
		Seq:            seq,
		MsgType:        model.MsgTypeSystem,
		Content:        content,
		ClientMsgID:    clientMsgID,
	}
	if err := s.msgs.Insert(ctx, msg); err != nil {
		return err
	}
	toUsers, err := s.resolveRecipients(ctx, convID, 0, 0)
	if err != nil {
		return err
	}
	evt := store.SentEvent{
		MsgID:          msg.ID,
		ConversationID: convID,
		FromUserID:     0,
		ToUserIDs:      toUsers,
		Seq:            seq,
		MsgType:        msg.MsgType,
		Content:        msg.Content,
		ClientMsgID:    clientMsgID,
		OriginInstance: s.instanceID,
	}
	if s.onSent != nil {
		s.onSent(ctx, &evt)
	}
	if err := pkgkafka.Publish(ctx, s.writer, []byte(convID), evt.Bytes()); err != nil {
		log.Printf("kafka publish system: %v", err)
	}
	return nil
}

func (s *MessageService) resolveRecipients(ctx context.Context, convID string, from int64, toUserID int64) ([]int64, error) {
	if strings.Contains(convID, "_") && !strings.HasPrefix(convID, "g_") {
		parts := strings.Split(convID, "_")
		if len(parts) != 2 {
			return nil, errors.New("会话 ID 无效")
		}
		a, _ := strconv.ParseInt(parts[0], 10, 64)
		b, _ := strconv.ParseInt(parts[1], 10, 64)
		var other int64
		if a == from {
			other = b
		} else {
			other = a
		}
		return []int64{other}, nil
	}
	if strings.HasPrefix(convID, "g_") {
		gid, _ := strconv.ParseInt(strings.TrimPrefix(convID, "g_"), 10, 64)
		return s.friend.ListGroupMemberIDs(ctx, gid)
	}
	if toUserID > 0 {
		return []int64{toUserID}, nil
	}
		return nil, errors.New("找不到消息接收人")
}

func (s *MessageService) HandleTyping(ctx context.Context, fromUserID int64, deviceID string, raw json.RawMessage) error {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Typing         *bool  `json:"typing"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return err
	}
	if req.ConversationID == "" {
		return errors.New("会话不存在")
	}
	if err := s.ensureMember(ctx, req.ConversationID, fromUserID); err != nil {
		return err
	}
	typing := true
	if req.Typing != nil {
		typing = *req.Typing
	}
	toUsers, err := s.resolveRecipients(ctx, req.ConversationID, fromUserID, 0)
	if err != nil {
		return err
	}
	if s.onTyping != nil {
		s.onTyping(ctx, &TypingEvent{
			ConversationID: req.ConversationID,
			FromUserID:     fromUserID,
			Typing:         typing,
			ToUserIDs:      toUsers,
		})
	}
	_ = deviceID
	return nil
}

func (s *MessageService) ensureMember(ctx context.Context, convID string, userID int64) error {
	ok, err := s.msgs.IsConversationMember(ctx, convID, userID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// Direct chats are created lazily on first send; allow friends to open empty history.
	if strings.Contains(convID, "_") && !strings.HasPrefix(convID, "g_") {
		parts := strings.Split(convID, "_")
		if len(parts) == 2 {
			a, errA := strconv.ParseInt(parts[0], 10, 64)
			b, errB := strconv.ParseInt(parts[1], 10, 64)
			if errA == nil && errB == nil && (userID == a || userID == b) {
				friends, ferr := s.friend.AreFriends(ctx, a, b)
				if ferr != nil {
					return ferr
				}
				if friends {
					_ = s.msgs.EnsureConversation(ctx, convID, model.ConvTypeDirect, []int64{a, b})
					return nil
				}
			}
		}
	}
	return errors.New("无权查看该会话")
}

func (s *MessageService) GetMessageForMember(ctx context.Context, userID int64, convID string, msgID int64) (*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, err
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *MessageService) ensureCanPost(ctx context.Context, convID string, userID int64) error {
	if !strings.HasPrefix(convID, "g_") {
		return nil
	}
	gidStr := strings.TrimPrefix(convID, "g_")
	groupID, err := strconv.ParseInt(gidStr, 10, 64)
	if err != nil || groupID <= 0 {
		return nil
	}
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return err
	}
	muted, err := s.friend.IsMemberMuted(ctx, groupID, userID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if muted {
		return errors.New("你已被禁言，无法发送消息")
	}
	role, err := s.friend.GetMemberRole(ctx, groupID, userID)
	if err != nil {
		return errors.New("你不在该群中")
	}
	isManager := role >= store.GroupRoleAdmin || g.OwnerID == userID
	if g.AdminOnly && !isManager {
		return errors.New("全员禁言中，仅管理员可发言")
	}
	if g.SlowModeSecs > 0 && !isManager {
		last, ok, lerr := s.msgs.LastUserMessageAt(ctx, convID, userID)
		if lerr != nil {
			return lerr
		}
		if ok {
			wait := time.Duration(g.SlowModeSecs) * time.Second
			elapsed := time.Since(last)
			if elapsed < wait {
				remain := int((wait - elapsed).Seconds()) + 1
				if remain < 1 {
					remain = 1
				}
				return fmt.Errorf("慢速模式：请 %d 秒后再发送", remain)
			}
		}
	}
	return nil
}

func (s *MessageService) ListMessages(ctx context.Context, userID int64, convID string, beforeSeq, aroundSeq int64, limit int) ([]*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if aroundSeq > 0 {
		return s.msgs.ListAround(ctx, convID, aroundSeq, limit)
	}
	msgs, err := s.msgs.ListByConversation(ctx, convID, beforeSeq, limit)
	if err != nil {
		return nil, err
	}
	// return chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

var allowedReactionEmojis = map[string]bool{
	"👍": true, "❤️": true, "😂": true, "😮": true, "😢": true, "🎉": true,
}

func (s *MessageService) ReactionsForMessages(ctx context.Context, viewerID int64, msgs []*model.Message) (map[string][]store.ReactionSummary, error) {
	ids := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		if m != nil && m.ID > 0 {
			ids = append(ids, m.ID)
		}
	}
	rows, err := s.msgs.ListReactionsForMessages(ctx, ids)
	if err != nil {
		return nil, err
	}
	agg := store.AggregateReactions(rows, viewerID)
	out := map[string][]store.ReactionSummary{}
	for msgID, list := range agg {
		out[strconv.FormatInt(msgID, 10)] = list
	}
	return out, nil
}

func (s *MessageService) ToggleReaction(ctx context.Context, userID int64, convID string, msgID int64, emoji string) ([]store.ReactionSummary, error) {
	if !allowedReactionEmojis[emoji] {
		return nil, errors.New("不支持的表情")
	}
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg.MsgType == model.MsgTypeSystem {
		return nil, errors.New("系统消息不能回应")
	}
	added, err := s.msgs.ToggleReaction(ctx, convID, msgID, userID, emoji)
	if err != nil {
		return nil, err
	}
	rows, err := s.msgs.ListReactionsForMessages(ctx, []int64{msgID})
	if err != nil {
		return nil, err
	}
	summaries := store.AggregateReactions(rows, userID)[msgID]
	if summaries == nil {
		summaries = []store.ReactionSummary{}
	}
	toUsers, _ := s.resolveRecipients(ctx, convID, userID, 0)
	if s.onReaction != nil {
		s.onReaction(ctx, &ReactionEvent{
			ConversationID: convID,
			MsgID:          msgID,
			UserID:         userID,
			Emoji:          emoji,
			Added:          added,
			ToUserIDs:      toUsers,
			Summaries:      summaries,
		})
	}
	return summaries, nil
}

func (s *MessageService) ListPins(ctx context.Context, userID int64, convID string) ([]store.PinnedMessage, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权查看该会话")
	}
	pins, err := s.msgs.ListPins(ctx, convID)
	if err != nil {
		return nil, err
	}
	if pins == nil {
		pins = []store.PinnedMessage{}
	}
	return pins, nil
}

func (s *MessageService) PinMessage(ctx context.Context, userID int64, convID string, msgID int64) ([]store.PinnedMessage, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg.MsgType == model.MsgTypeSystem || msg.Content == "[已撤回]" {
		return nil, errors.New("该消息不可置顶")
	}
	already, err := s.msgs.IsPinned(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if !already {
		n, err := s.msgs.CountPins(ctx, convID)
		if err != nil {
			return nil, err
		}
		if n >= store.MaxConversationPins {
			return nil, fmt.Errorf("每个会话最多置顶 %d 条消息", store.MaxConversationPins)
		}
	}
	if err := s.msgs.PinMessage(ctx, convID, msgID, userID); err != nil {
		return nil, err
	}
	pins, err := s.msgs.ListPins(ctx, convID)
	if err != nil {
		return nil, err
	}
	if pins == nil {
		pins = []store.PinnedMessage{}
	}
	toUsers, _ := s.resolveRecipients(ctx, convID, userID, 0)
	if s.onPin != nil {
		s.onPin(ctx, &PinEvent{
			ConversationID: convID,
			MsgID:          msgID,
			UserID:         userID,
			Pinned:         true,
			ToUserIDs:      toUsers,
			Pins:           pins,
		})
	}
	return pins, nil
}

func (s *MessageService) UnpinMessage(ctx context.Context, userID int64, convID string, msgID int64) ([]store.PinnedMessage, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	if err := s.msgs.UnpinMessage(ctx, convID, msgID); err != nil {
		return nil, err
	}
	pins, err := s.msgs.ListPins(ctx, convID)
	if err != nil {
		return nil, err
	}
	if pins == nil {
		pins = []store.PinnedMessage{}
	}
	toUsers, _ := s.resolveRecipients(ctx, convID, userID, 0)
	if s.onPin != nil {
		s.onPin(ctx, &PinEvent{
			ConversationID: convID,
			MsgID:          msgID,
			UserID:         userID,
			Pinned:         false,
			ToUserIDs:      toUsers,
			Pins:           pins,
		})
	}
	return pins, nil
}

func (s *MessageService) ListBookmarks(ctx context.Context, userID int64, convID string) ([]store.ConversationBookmark, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权查看该会话")
	}
	list, err := s.msgs.ListBookmarks(ctx, convID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.ConversationBookmark{}
	}
	return list, nil
}

func (s *MessageService) AddBookmark(ctx context.Context, userID int64, convID, title, rawURL string) (*store.ConversationBookmark, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	title = strings.TrimSpace(title)
	rawURL = strings.TrimSpace(rawURL)
	if title == "" || rawURL == "" {
		return nil, errors.New("标题和链接不能为空")
	}
	if len([]rune(title)) > 64 {
		return nil, errors.New("标题过长")
	}
	if len(rawURL) > 1024 {
		return nil, errors.New("链接过长")
	}
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return nil, errors.New("仅支持 http/https 链接")
	}
	n, err := s.msgs.CountBookmarks(ctx, convID)
	if err != nil {
		return nil, err
	}
	if n >= store.MaxConversationBookmarks {
		return nil, fmt.Errorf("每个会话最多 %d 个书签", store.MaxConversationBookmarks)
	}
	return s.msgs.AddBookmark(ctx, convID, title, rawURL, userID)
}

func (s *MessageService) DeleteBookmark(ctx context.Context, userID int64, convID string, bookmarkID int64) error {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return errors.New("无权操作该会话")
	}
	return s.msgs.DeleteBookmark(ctx, convID, bookmarkID)
}

func (s *MessageService) ListStarred(ctx context.Context, userID int64, limit int) ([]store.StarredMessage, error) {
	list, err := s.msgs.ListStarred(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.StarredMessage{}
	}
	return list, nil
}

func (s *MessageService) ToggleStar(ctx context.Context, userID int64, convID string, msgID int64) (starred bool, err error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return false, errors.New("无权操作该会话")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return false, err
	}
	if msg.MsgType == model.MsgTypeSystem {
		return false, errors.New("系统消息不可收藏")
	}
	ok, err := s.msgs.IsStarred(ctx, userID, msgID)
	if err != nil {
		return false, err
	}
	if ok {
		if err := s.msgs.UnstarMessage(ctx, userID, msgID); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.msgs.StarMessage(ctx, userID, convID, msgID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MessageService) StarredFlags(ctx context.Context, userID int64, msgs []*model.Message) (map[string]bool, error) {
	ids := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	flags, err := s.msgs.ListStarredIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for id, v := range flags {
		if v {
			out[strconv.FormatInt(id, 10)] = true
		}
	}
	return out, nil
}

func (s *MessageService) SearchMessages(ctx context.Context, userID int64, convID, keyword string, beforeSeq int64, limit int) ([]*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("请输入搜索关键词")
	}
	if len([]rune(keyword)) > 64 {
		return nil, errors.New("搜索关键词过长")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	msgs, err := s.msgs.SearchInConversation(ctx, convID, keyword, beforeSeq, limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *MessageService) SearchGlobal(ctx context.Context, userID int64, keyword string, limit int) ([]*model.Message, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("请输入搜索关键词")
	}
	if len([]rune(keyword)) > 64 {
		return nil, errors.New("搜索关键词过长")
	}
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	return s.msgs.SearchGlobal(ctx, userID, keyword, limit)
}

func (s *MessageService) ListConversations(ctx context.Context, userID int64) ([]store.ConversationSummary, error) {
	return s.msgs.ListConversations(ctx, userID)
}

func (s *MessageService) ExportTranscript(ctx context.Context, userID int64, convID string) (string, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return "", errors.New("无权查看该会话")
	}
	msgs, err := s.msgs.ListForExport(ctx, convID, 2000)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# SquirtleChat export\n")
	b.WriteString("# conversation_id=" + convID + "\n")
	b.WriteString("# exported_at=" + time.Now().Format(time.RFC3339) + "\n\n")
	for _, m := range msgs {
		who := strconv.FormatInt(m.FromUserID, 10)
		ts := m.CreatedAt.Format("2006-01-02 15:04:05")
		line := m.Content
		switch m.MsgType {
		case model.MsgTypeImage:
			line = "[图片]"
		case model.MsgTypeFile:
			line = "[文件]"
		case model.MsgTypeAudio:
			line = "[语音]"
		case model.MsgTypeSystem:
			line = m.Content
		}
		if m.EditedAt != nil {
			line += " (已编辑)"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", ts, who, line)
	}
	return b.String(), nil
}

func (s *MessageService) ListUnreadMentions(ctx context.Context, userID int64, patterns []string, limit int) ([]*model.Message, error) {
	msgs, err := s.msgs.ListUnreadMentions(ctx, userID, patterns, limit)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	return msgs, nil
}

type pollPayload struct {
	Question string `json:"question"`
	Options  []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"options"`
}

func validatePollContent(content string) error {
	var p pollPayload
	if err := json.Unmarshal([]byte(content), &p); err != nil {
		return errors.New("投票内容无效")
	}
	q := strings.TrimSpace(p.Question)
	if q == "" || utf8.RuneCountInString(q) > 120 {
		return errors.New("投票问题无效")
	}
	if len(p.Options) < 2 || len(p.Options) > 8 {
		return errors.New("投票选项需 2–8 个")
	}
	seen := map[string]struct{}{}
	for _, o := range p.Options {
		id := strings.TrimSpace(o.ID)
		text := strings.TrimSpace(o.Text)
		if id == "" || text == "" || utf8.RuneCountInString(text) > 64 {
			return errors.New("投票选项无效")
		}
		if _, ok := seen[id]; ok {
			return errors.New("投票选项重复")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (s *MessageService) VotePoll(ctx context.Context, userID int64, convID string, msgID int64, optionID string) (*store.PollResult, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	optionID = strings.TrimSpace(optionID)
	if optionID == "" {
		return nil, errors.New("请选择选项")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg.MsgType != model.MsgTypePoll {
		return nil, errors.New("该消息不是投票")
	}
	var p pollPayload
	if err := json.Unmarshal([]byte(msg.Content), &p); err != nil {
		return nil, errors.New("投票内容无效")
	}
	ok := false
	for _, o := range p.Options {
		if o.ID == optionID {
			ok = true
			break
		}
	}
	if !ok {
		return nil, errors.New("选项不存在")
	}
	if err := s.msgs.UpsertPollVote(ctx, convID, msgID, userID, optionID); err != nil {
		return nil, err
	}
	res, err := s.msgs.PollResults(ctx, msgID, userID)
	if err != nil {
		return nil, err
	}
	toUsers, _ := s.resolveRecipients(ctx, convID, userID, 0)
	if s.onPollVote != nil {
		s.onPollVote(ctx, &PollVoteEvent{
			ConversationID: convID,
			MsgID:          msgID,
			UserID:         userID,
			OptionID:       optionID,
			ToUserIDs:      toUsers,
			Result:         res,
		})
	}
	return res, nil
}

func (s *MessageService) PollsForMessages(ctx context.Context, viewerID int64, msgs []*model.Message) (map[string]*store.PollResult, error) {
	ids := make([]int64, 0)
	for _, m := range msgs {
		if m != nil && m.MsgType == model.MsgTypePoll {
			ids = append(ids, m.ID)
		}
	}
	raw, err := s.msgs.PollResultsForMessages(ctx, viewerID, ids)
	if err != nil {
		return nil, err
	}
	out := map[string]*store.PollResult{}
	for id, r := range raw {
		out[strconv.FormatInt(id, 10)] = r
	}
	return out, nil
}

func (s *MessageService) RecallMessage(ctx context.Context, userID int64, convID string, msgID int64) (*model.Message, error) {
	ok, err := s.msgs.IsConversationMember(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("无权操作该会话")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg.FromUserID != userID {
		return nil, errors.New("只能撤回自己的消息")
	}
	if msg.MsgType == model.MsgTypeSystem {
		return nil, errors.New("消息已撤回")
	}
	if time.Since(msg.CreatedAt) > 2*time.Minute {
		return nil, errors.New("超过2分钟无法撤回")
	}
	if err := s.msgs.Recall(ctx, convID, msgID); err != nil {
		return nil, err
	}
	_ = s.msgs.DeleteHashtagsForMessage(ctx, msgID)
	msg.MsgType = model.MsgTypeSystem
	msg.Content = "[已撤回]"
	if s.onRecalled != nil {
		toUsers, err := s.resolveRecipients(ctx, convID, userID, 0)
		if err != nil {
			return msg, nil
		}
		s.onRecalled(ctx, &store.RecallEvent{
			MsgID:          msg.ID,
			ConversationID: convID,
			FromUserID:     userID,
			ClientMsgID:    msg.ClientMsgID,
			Seq:            msg.Seq,
			ToUserIDs:      toUsers,
		})
	}
	return msg, nil
}

const editWindow = 15 * time.Minute
const maxEditLen = 4000

func (s *MessageService) EditMessage(ctx context.Context, userID int64, convID string, msgID int64, content string) (*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("内容不能为空")
	}
	if utf8.RuneCountInString(content) > maxEditLen {
		return nil, errors.New("内容过长")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg.FromUserID != userID {
		return nil, errors.New("只能编辑自己的消息")
	}
	if msg.MsgType != model.MsgTypeText {
		return nil, errors.New("仅支持编辑文本消息")
	}
	if time.Since(msg.CreatedAt) > editWindow {
		return nil, errors.New("超过15分钟无法编辑")
	}
	if err := s.msgs.UpdateTextContent(ctx, convID, msgID, content); err != nil {
		return nil, err
	}
	_ = s.msgs.ReplaceHashtags(ctx, convID, msgID, extractHashtags(content))
	updated, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	toUsers, _ := s.resolveRecipients(ctx, convID, userID, 0)
	if s.onEdited != nil {
		s.onEdited(ctx, &EditEvent{Message: updated, ToUserIDs: toUsers})
	}
	return updated, nil
}

func (s *MessageService) ScheduleMessage(ctx context.Context, userID int64, convID string, convType int8, toUserID int64, content string, sendAt time.Time) (*store.ScheduledMessage, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("内容不能为空")
	}
	if utf8.RuneCountInString(content) > maxEditLen {
		return nil, errors.New("内容过长")
	}
	if sendAt.Before(time.Now().Add(30 * time.Second)) {
		return nil, errors.New("发送时间至少在 30 秒之后")
	}
	if sendAt.After(time.Now().Add(30 * 24 * time.Hour)) {
		return nil, errors.New("最多提前 30 天定时")
	}
	if convType == 0 {
		convType = model.ConvTypeDirect
	}
	m := &store.ScheduledMessage{
		UserID:           userID,
		ConversationID:   convID,
		ConversationType: convType,
		ToUserID:         toUserID,
		Content:          content,
		MsgType:          model.MsgTypeText,
		SendAt:           sendAt,
	}
	if err := s.msgs.InsertScheduled(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MessageService) ListScheduled(ctx context.Context, userID int64) ([]store.ScheduledMessage, error) {
	list, err := s.msgs.ListScheduled(ctx, userID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.ScheduledMessage{}
	}
	return list, nil
}

func (s *MessageService) CancelScheduled(ctx context.Context, userID, id int64) error {
	return s.msgs.CancelScheduled(ctx, userID, id)
}

func (s *MessageService) RunScheduleWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushScheduled(ctx)
			s.flushReminders(ctx)
		}
	}
}

func (s *MessageService) flushScheduled(ctx context.Context) {
	due, err := s.msgs.ClaimDueScheduled(ctx, 20)
	if err != nil || len(due) == 0 {
		return
	}
	for _, item := range due {
		payload, _ := json.Marshal(sendReq{
			ClientMsgID:      fmt.Sprintf("sched-%d-%d", item.ID, time.Now().UnixNano()),
			ConversationID:   item.ConversationID,
			ConversationType: item.ConversationType,
			ToUserID:         strconv.FormatInt(item.ToUserID, 10),
			MsgType:          item.MsgType,
			Content:          item.Content,
		})
		if _, err := s.HandleSend(ctx, item.UserID, "scheduler", payload); err != nil {
			log.Printf("scheduled send id=%d: %v", item.ID, err)
		}
	}
}

func reminderPreview(content string, msgType int8) string {
	text := strings.TrimSpace(content)
	switch msgType {
	case model.MsgTypeImage:
		return "[图片]"
	case model.MsgTypeFile:
		return "[文件]"
	case model.MsgTypeAudio:
		return "[语音]"
	case model.MsgTypePoll:
		return "[投票]"
	case model.MsgTypeSystem:
		return "[系统]"
	}
	runes := []rune(text)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return text
}

func (s *MessageService) CreateReminder(ctx context.Context, userID int64, convID string, msgID int64, remindAt time.Time) (*store.MessageReminder, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权操作该会话")
	}
	msg, err := s.msgs.GetByID(ctx, convID, msgID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, store.ErrNotFound
	}
	if msg.MsgType == model.MsgTypeSystem {
		return nil, errors.New("系统消息不可提醒")
	}
	if remindAt.Before(time.Now().Add(30 * time.Second)) {
		return nil, errors.New("提醒时间至少在 30 秒之后")
	}
	if remindAt.After(time.Now().Add(30 * 24 * time.Hour)) {
		return nil, errors.New("最多提前 30 天提醒")
	}
	r := &store.MessageReminder{
		UserID:         userID,
		ConversationID: convID,
		MsgID:          msgID,
		Preview:        reminderPreview(msg.Content, msg.MsgType),
		RemindAt:       remindAt,
	}
	if err := s.msgs.InsertReminder(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *MessageService) ListReminders(ctx context.Context, userID int64) ([]store.MessageReminder, error) {
	list, err := s.msgs.ListReminders(ctx, userID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.MessageReminder{}
	}
	return list, nil
}

func (s *MessageService) CancelReminder(ctx context.Context, userID, id int64) error {
	return s.msgs.CancelReminder(ctx, userID, id)
}

func (s *MessageService) flushReminders(ctx context.Context) {
	due, err := s.msgs.ClaimDueReminders(ctx, 20)
	if err != nil || len(due) == 0 {
		return
	}
	for i := range due {
		item := due[i]
		if s.onReminder != nil {
			s.onReminder(ctx, &ReminderEvent{Reminder: &item})
		}
	}
}

var hashtagRe = regexp.MustCompile(`(?i)(?:^|[\s([{（【「『])#([a-z0-9_\p{Han}]{1,64})`)

const replyStart = "⟦sq-reply⟧"
const replyEnd = "⟦/sq-reply⟧"

func plainMessageText(content string) string {
	if !strings.HasPrefix(content, replyStart) {
		return content
	}
	end := strings.Index(content, replyEnd)
	if end < 0 {
		return content
	}
	text := content[end+len(replyEnd):]
	return strings.TrimPrefix(text, "\n")
}

func extractHashtags(content string) []string {
	text := plainMessageText(content)
	matches := hashtagRe.FindAllStringSubmatch(text, 20)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		tag := strings.ToLower(m[1])
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func (s *MessageService) ListHashtags(ctx context.Context, userID int64, convID string, limit int) ([]store.HashtagCount, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权查看该会话")
	}
	list, err := s.msgs.ListHashtags(ctx, convID, limit)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.HashtagCount{}
	}
	return list, nil
}

func (s *MessageService) SearchByHashtag(ctx context.Context, userID int64, convID, tag string, beforeSeq int64, limit int) ([]*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权查看该会话")
	}
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	if tag == "" {
		return nil, errors.New("请输入话题标签")
	}
	if utf8.RuneCountInString(tag) > 64 {
		return nil, errors.New("标签过长")
	}
	list, err := s.msgs.ListByHashtag(ctx, convID, tag, beforeSeq, limit)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*model.Message{}
	}
	return list, nil
}

func (s *MessageService) ListMedia(ctx context.Context, userID int64, convID, kind string, beforeSeq int64, limit int) ([]*model.Message, error) {
	if err := s.ensureMember(ctx, convID, userID); err != nil {
		return nil, errors.New("无权查看该会话")
	}
	list, err := s.msgs.ListMedia(ctx, convID, kind, beforeSeq, limit)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*model.Message{}
	}
	return list, nil
}
