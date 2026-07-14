package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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
}

type TypingEvent struct {
	ConversationID string
	FromUserID     int64
	Typing         bool
	ToUserIDs      []int64
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
	if err := s.msgs.Insert(ctx, msg); err != nil {
		return nil, err
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

func (s *MessageService) ListConversations(ctx context.Context, userID int64) ([]store.ConversationSummary, error) {
	return s.msgs.ListConversations(ctx, userID)
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
