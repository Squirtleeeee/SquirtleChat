package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"squirtlechat/internal/model"
)

type MessageStore struct {
	db *sql.DB
}

func NewMessageStore(db *sql.DB) *MessageStore {
	return &MessageStore{db: db}
}

const messageSelectCols = `id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at, edited_at`

func scanMessageRow(scanner interface{ Scan(dest ...any) error }) (*model.Message, error) {
	m := &model.Message{}
	var edited sql.NullTime
	err := scanner.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt, &edited)
	if err != nil {
		return nil, err
	}
	if edited.Valid {
		t := edited.Time
		m.EditedAt = &t
	}
	return m, nil
}

func (s *MessageStore) NextSeq(ctx context.Context, convID string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var seq int64
	row := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0)+1 FROM messages WHERE conversation_id = ? FOR UPDATE`, convID)
	if err := row.Scan(&seq); err != nil {
		return 0, err
	}
	return seq, tx.Commit()
}

func (s *MessageStore) GetByID(ctx context.Context, convID string, msgID int64) (*model.Message, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+messageSelectCols+`
		FROM messages WHERE conversation_id = ? AND id = ?`, convID, msgID)
	m, err := scanMessageRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *MessageStore) Recall(ctx context.Context, convID string, msgID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE messages SET msg_type = ?, content = ?, edited_at = NULL
		WHERE id = ? AND conversation_id = ? AND msg_type != ?`,
		model.MsgTypeSystem, "[已撤回]", msgID, convID, model.MsgTypeSystem)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MessageStore) UpdateTextContent(ctx context.Context, convID string, msgID int64, content string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE messages SET content = ?, edited_at = NOW(3)
		WHERE id = ? AND conversation_id = ? AND msg_type = ?`,
		content, msgID, convID, model.MsgTypeText)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE conversations SET updated_at = NOW(3) WHERE id = ?`, convID)
	return nil
}

func (s *MessageStore) Insert(ctx context.Context, m *model.Message) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.FromUserID, m.Seq, m.MsgType, m.Content, m.ClientMsgID)
	if err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE conversations SET updated_at = NOW(3) WHERE id = ?`, m.ConversationID)
	return nil
}

func (s *MessageStore) LastUserMessageAt(ctx context.Context, convID string, userID int64) (time.Time, bool, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT created_at FROM messages
		WHERE conversation_id = ? AND from_user_id = ?
		ORDER BY seq DESC LIMIT 1`, convID, userID).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

func (s *MessageStore) GetByClientMsgID(ctx context.Context, convID, clientMsgID string) (*model.Message, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+messageSelectCols+`
		FROM messages WHERE conversation_id = ? AND client_msg_id = ?`, convID, clientMsgID)
	m, err := scanMessageRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *MessageStore) ListByConversation(ctx context.Context, convID string, beforeSeq int64, limit int) ([]*model.Message, error) {
	q := `SELECT ` + messageSelectCols + `
		FROM messages WHERE conversation_id = ?`
	args := []interface{}{convID}
	if beforeSeq > 0 {
		q += ` AND seq < ?`
		args = append(args, beforeSeq)
	}
	q += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) ListMedia(ctx context.Context, convID, kind string, beforeSeq int64, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	var types []int8
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image":
		types = []int8{model.MsgTypeImage}
	case "file":
		types = []int8{model.MsgTypeFile}
	case "voice", "audio":
		types = []int8{model.MsgTypeAudio}
	default:
		types = []int8{model.MsgTypeImage, model.MsgTypeFile, model.MsgTypeAudio}
	}
	placeholders := make([]string, len(types))
	args := []interface{}{convID}
	for i, t := range types {
		placeholders[i] = "?"
		args = append(args, t)
	}
	q := `SELECT ` + messageSelectCols + `
		FROM messages
		WHERE conversation_id = ? AND msg_type IN (` + strings.Join(placeholders, ",") + `)`
	if beforeSeq > 0 {
		q += ` AND seq < ?`
		args = append(args, beforeSeq)
	}
	q += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) SearchInConversation(ctx context.Context, convID, keyword string, beforeSeq int64, limit int) ([]*model.Message, error) {
	q := `SELECT ` + messageSelectCols + `
		FROM messages
		WHERE conversation_id = ? AND msg_type = ? AND content LIKE ?`
	args := []interface{}{convID, model.MsgTypeText, "%" + keyword + "%"}
	if beforeSeq > 0 {
		q += ` AND seq < ?`
		args = append(args, beforeSeq)
	}
	q += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) SearchGlobal(ctx context.Context, userID int64, keyword string, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at, m.edited_at
		FROM messages m
		INNER JOIN conversation_members cm
			ON cm.conversation_id = m.conversation_id AND cm.user_id = ?
		WHERE m.msg_type = ? AND m.content LIKE ?
		ORDER BY m.created_at DESC
		LIMIT ?`,
		userID, model.MsgTypeText, "%"+keyword+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ListAround returns messages centered on centerSeq (chronological order).
func (s *MessageStore) ListAround(ctx context.Context, convID string, centerSeq int64, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	beforeHalf := limit / 2
	startSeq := centerSeq - int64(beforeHalf)
	if startSeq < 1 {
		startSeq = 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+messageSelectCols+`
		FROM messages
		WHERE conversation_id = ? AND seq >= ?
		ORDER BY seq ASC
		LIMIT ?`, convID, startSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) EnsureConversation(ctx context.Context, convID string, convType int8, members []int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO conversations (id, type) VALUES (?, ?)`, convID, convType)
	if err != nil {
		return err
	}
	for _, uid := range members {
		_, err := s.db.ExecContext(ctx, `
			INSERT IGNORE INTO conversation_members (conversation_id, user_id) VALUES (?, ?)`, convID, uid)
		if err != nil {
			return err
		}
	}
	return nil
}

type ConversationSummary struct {
	ID          string    `json:"conversation_id"`
	Type        int8      `json:"type"`
	Title       string    `json:"title"`
	LastSeq     int64     `json:"last_seq"`
	LastReadSeq int64     `json:"last_read_seq"`
	UnreadCount int64     `json:"unread_count"`
	LastContent string    `json:"last_content"`
	LastMsgType int8      `json:"last_msg_type"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *MessageStore) ListConversations(ctx context.Context, userID int64) ([]ConversationSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.type, cm.last_read_seq,
			COALESCE((SELECT MAX(seq) FROM messages m WHERE m.conversation_id = c.id), 0),
			COALESCE((SELECT content FROM messages m WHERE m.conversation_id = c.id ORDER BY seq DESC LIMIT 1), ''),
			COALESCE((SELECT msg_type FROM messages m WHERE m.conversation_id = c.id ORDER BY seq DESC LIMIT 1), 0),
			COALESCE((SELECT created_at FROM messages m WHERE m.conversation_id = c.id ORDER BY seq DESC LIMIT 1), c.updated_at)
		FROM conversations c
		INNER JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = ?
		ORDER BY COALESCE(
			(SELECT created_at FROM messages m WHERE m.conversation_id = c.id ORDER BY seq DESC LIMIT 1),
			c.updated_at
		) DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ConversationSummary
	for rows.Next() {
		var item ConversationSummary
		if err := rows.Scan(&item.ID, &item.Type, &item.LastReadSeq, &item.LastSeq, &item.LastContent, &item.LastMsgType, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if item.LastSeq > item.LastReadSeq {
			item.UnreadCount = item.LastSeq - item.LastReadSeq
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *MessageStore) BumpLastReadSeq(ctx context.Context, convID string, userID, seq int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE conversation_members SET last_read_seq = GREATEST(last_read_seq, ?)
		WHERE conversation_id = ? AND user_id = ?`, seq, convID, userID)
	return err
}

func (s *MessageStore) GetMaxSeq(ctx context.Context, convID string) (int64, error) {
	var seq int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM messages WHERE conversation_id = ?`, convID).Scan(&seq)
	return seq, err
}

func (s *MessageStore) ListForExport(ctx context.Context, convID string, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+messageSelectCols+`
		FROM messages
		WHERE conversation_id = ?
		ORDER BY seq ASC
		LIMIT ?`, convID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) ListUnreadMentions(ctx context.Context, userID int64, patterns []string, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	if len(patterns) == 0 {
		return nil, nil
	}
	like := make([]string, len(patterns))
	args := []interface{}{userID, userID, model.MsgTypeText}
	for i, p := range patterns {
		like[i] = `m.content LIKE ?`
		args = append(args, "%"+p+"%")
	}
	args = append(args, limit)
	q := `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at, m.edited_at
		FROM messages m
		INNER JOIN conversation_members cm
			ON cm.conversation_id = m.conversation_id AND cm.user_id = ?
		WHERE m.from_user_id != ?
			AND m.msg_type = ?
			AND m.seq > cm.last_read_seq
			AND (` + strings.Join(like, " OR ") + `)
		ORDER BY m.created_at DESC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) ListOfflineMessages(ctx context.Context, userID int64) ([]*model.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at, m.edited_at
		FROM offline_inbox o
		INNER JOIN messages m ON m.id = o.msg_id
		WHERE o.user_id = ?
		ORDER BY m.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) ClearOffline(ctx context.Context, userID int64, msgIDs []int64) error {
	if len(msgIDs) == 0 {
		return nil
	}
	q := `DELETE FROM offline_inbox WHERE user_id = ? AND msg_id IN (?` + strings.Repeat(",?", len(msgIDs)-1) + `)`
	args := make([]interface{}, 0, len(msgIDs)+1)
	args = append(args, userID)
	for _, id := range msgIDs {
		args = append(args, id)
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *MessageStore) AddOffline(ctx context.Context, userID, msgID int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO offline_inbox (user_id, msg_id) VALUES (?, ?)`, userID, msgID)
	return err
}

type MemberReadSeq struct {
	UserID  int64
	ReadSeq int64
}

func (s *MessageStore) ListMemberReadSeqs(ctx context.Context, convID string) ([]MemberReadSeq, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, last_read_seq FROM conversation_members WHERE conversation_id = ?`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MemberReadSeq
	for rows.Next() {
		var m MemberReadSeq
		if err := rows.Scan(&m.UserID, &m.ReadSeq); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) IsConversationMember(ctx context.Context, convID string, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM conversation_members WHERE conversation_id = ? AND user_id = ?`, convID, userID).Scan(&n)
	return n > 0, err
}

type SentEvent struct {
	MsgID          int64  `json:"msg_id"`
	ConversationID string `json:"conversation_id"`
	FromUserID     int64  `json:"from_user_id"`
	ToUserIDs      []int64 `json:"to_user_ids"`
	Seq            int64  `json:"seq"`
	MsgType        int8   `json:"msg_type"`
	Content        string `json:"content"`
	ClientMsgID    string `json:"client_msg_id"`
	ExceptDevice   string `json:"except_device"`
	OriginInstance string `json:"origin_instance,omitempty"`
}

func (e *SentEvent) Bytes() []byte {
	b, _ := json.Marshal(e)
	return b
}

type RecallEvent struct {
	MsgID          int64   `json:"msg_id"`
	ConversationID string  `json:"conversation_id"`
	FromUserID     int64   `json:"from_user_id"`
	ClientMsgID    string  `json:"client_msg_id"`
	Seq            int64   `json:"seq"`
	ToUserIDs      []int64 `json:"to_user_ids"`
}

type ReactionRow struct {
	MsgID  int64
	UserID int64
	Emoji  string
}

type ReactionSummary struct {
	Emoji   string  `json:"emoji"`
	Count   int     `json:"count"`
	UserIDs []int64 `json:"user_ids"`
	Mine    bool    `json:"mine"`
}

func (s *MessageStore) ToggleReaction(ctx context.Context, convID string, msgID, userID int64, emoji string) (added bool, err error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM message_reactions WHERE msg_id = ? AND user_id = ? AND emoji = ?`,
		msgID, userID, emoji)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return false, nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO message_reactions (conversation_id, msg_id, user_id, emoji)
		VALUES (?, ?, ?, ?)`, convID, msgID, userID, emoji)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *MessageStore) ListReactionsForMessages(ctx context.Context, msgIDs []int64) ([]ReactionRow, error) {
	if len(msgIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, len(msgIDs))
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT msg_id, user_id, emoji FROM message_reactions WHERE msg_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReactionRow
	for rows.Next() {
		var r ReactionRow
		if err := rows.Scan(&r.MsgID, &r.UserID, &r.Emoji); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func AggregateReactions(rows []ReactionRow, viewerID int64) map[int64][]ReactionSummary {
	type key struct {
		msgID int64
		emoji string
	}
	users := map[key][]int64{}
	for _, r := range rows {
		k := key{r.MsgID, r.Emoji}
		users[k] = append(users[k], r.UserID)
	}
	out := map[int64][]ReactionSummary{}
	for k, ids := range users {
		mine := false
		for _, id := range ids {
			if id == viewerID {
				mine = true
				break
			}
		}
		out[k.msgID] = append(out[k.msgID], ReactionSummary{
			Emoji:   k.emoji,
			Count:   len(ids),
			UserIDs: ids,
			Mine:    mine,
		})
	}
	return out
}

type PinnedMessage struct {
	Message  *model.Message `json:"message"`
	PinnedBy int64          `json:"pinned_by,string"`
	PinnedAt time.Time      `json:"pinned_at"`
}

const MaxConversationPins = 50

func (s *MessageStore) CountPins(ctx context.Context, convID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM conversation_pins WHERE conversation_id = ?`, convID).Scan(&n)
	return n, err
}

func (s *MessageStore) PinMessage(ctx context.Context, convID string, msgID, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_pins (conversation_id, msg_id, pinned_by)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE pinned_by = VALUES(pinned_by), pinned_at = CURRENT_TIMESTAMP(3)`,
		convID, msgID, userID)
	return err
}

func (s *MessageStore) UnpinMessage(ctx context.Context, convID string, msgID int64) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM conversation_pins WHERE conversation_id = ? AND msg_id = ?`, convID, msgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MessageStore) IsPinned(ctx context.Context, convID string, msgID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM conversation_pins WHERE conversation_id = ? AND msg_id = ?`,
		convID, msgID).Scan(&n)
	return n > 0, err
}

func (s *MessageStore) ListPins(ctx context.Context, convID string) ([]PinnedMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at, m.edited_at,
			p.pinned_by, p.pinned_at
		FROM conversation_pins p
		INNER JOIN messages m ON m.id = p.msg_id AND m.conversation_id = p.conversation_id
		WHERE p.conversation_id = ?
		ORDER BY p.pinned_at DESC
		LIMIT ?`, convID, MaxConversationPins)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PinnedMessage
	for rows.Next() {
		m := &model.Message{}
		var edited sql.NullTime
		var pm PinnedMessage
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt, &edited,
			&pm.PinnedBy, &pm.PinnedAt,
		); err != nil {
			return nil, err
		}
		if edited.Valid {
			t := edited.Time
			m.EditedAt = &t
		}
		pm.Message = m
		out = append(out, pm)
	}
	return out, rows.Err()
}

type ConversationBookmark struct {
	ID             int64     `json:"id,string"`
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	CreatedBy      int64     `json:"created_by,string"`
	CreatedAt      time.Time `json:"created_at"`
}

const MaxConversationBookmarks = 20

func (s *MessageStore) CountBookmarks(ctx context.Context, convID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM conversation_bookmarks WHERE conversation_id = ?`, convID).Scan(&n)
	return n, err
}

func (s *MessageStore) ListBookmarks(ctx context.Context, convID string) ([]ConversationBookmark, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, conversation_id, title, url, created_by, created_at
		FROM conversation_bookmarks
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, convID, MaxConversationBookmarks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConversationBookmark
	for rows.Next() {
		var b ConversationBookmark
		if err := rows.Scan(&b.ID, &b.ConversationID, &b.Title, &b.URL, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *MessageStore) AddBookmark(ctx context.Context, convID, title, url string, userID int64) (*ConversationBookmark, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_bookmarks (conversation_id, title, url, created_by)
		VALUES (?, ?, ?, ?)`, convID, title, url, userID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ConversationBookmark{
		ID:             id,
		ConversationID: convID,
		Title:          title,
		URL:            url,
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
	}, nil
}

func (s *MessageStore) DeleteBookmark(ctx context.Context, convID string, bookmarkID int64) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM conversation_bookmarks WHERE conversation_id = ? AND id = ?`, convID, bookmarkID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type ScheduledMessage struct {
	ID               int64     `json:"id,string"`
	UserID           int64     `json:"user_id,string"`
	ConversationID   string    `json:"conversation_id"`
	ConversationType int8      `json:"conversation_type"`
	ToUserID         int64     `json:"to_user_id,string"`
	Content          string    `json:"content"`
	MsgType          int8      `json:"msg_type"`
	SendAt           time.Time `json:"send_at"`
	Status           int8      `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

const (
	SchedPending   int8 = 0
	SchedSent      int8 = 1
	SchedCancelled int8 = 2
)

func (s *MessageStore) InsertScheduled(ctx context.Context, m *ScheduledMessage) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO scheduled_messages
			(user_id, conversation_id, conversation_type, to_user_id, content, msg_type, send_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.UserID, m.ConversationID, m.ConversationType, m.ToUserID, m.Content, m.MsgType, m.SendAt, SchedPending)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	m.Status = SchedPending
	return nil
}

func (s *MessageStore) ListScheduled(ctx context.Context, userID int64) ([]ScheduledMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, conversation_id, conversation_type, to_user_id, content, msg_type, send_at, status, created_at
		FROM scheduled_messages
		WHERE user_id = ? AND status = ?
		ORDER BY send_at ASC
		LIMIT 50`, userID, SchedPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledMessage
	for rows.Next() {
		var m ScheduledMessage
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.ConversationID, &m.ConversationType, &m.ToUserID,
			&m.Content, &m.MsgType, &m.SendAt, &m.Status, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MessageStore) CancelScheduled(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_messages SET status = ?
		WHERE id = ? AND user_id = ? AND status = ?`,
		SchedCancelled, id, userID, SchedPending)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MessageStore) ClaimDueScheduled(ctx context.Context, limit int) ([]ScheduledMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, conversation_id, conversation_type, to_user_id, content, msg_type, send_at, status, created_at
		FROM scheduled_messages
		WHERE status = ? AND send_at <= NOW(3)
		ORDER BY send_at ASC
		LIMIT ? FOR UPDATE`, SchedPending, limit)
	if err != nil {
		return nil, err
	}
	var out []ScheduledMessage
	var ids []int64
	for rows.Next() {
		var m ScheduledMessage
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.ConversationID, &m.ConversationType, &m.ToUserID,
			&m.Content, &m.MsgType, &m.SendAt, &m.Status, &m.CreatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, m)
		ids = append(ids, m.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE scheduled_messages SET status = ? WHERE id = ? AND status = ?`,
			SchedSent, id, SchedPending); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

type StarredMessage struct {
	Message   *model.Message `json:"message"`
	StarredAt time.Time      `json:"starred_at"`
}

func (s *MessageStore) StarMessage(ctx context.Context, userID int64, convID string, msgID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_starred_messages (user_id, conversation_id, msg_id)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE starred_at = CURRENT_TIMESTAMP(3)`,
		userID, convID, msgID)
	return err
}

func (s *MessageStore) UnstarMessage(ctx context.Context, userID, msgID int64) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM user_starred_messages WHERE user_id = ? AND msg_id = ?`, userID, msgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MessageStore) IsStarred(ctx context.Context, userID, msgID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_starred_messages WHERE user_id = ? AND msg_id = ?`,
		userID, msgID).Scan(&n)
	return n > 0, err
}

func (s *MessageStore) ListStarred(ctx context.Context, userID int64, limit int) ([]StarredMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at, m.edited_at,
			s.starred_at
		FROM user_starred_messages s
		INNER JOIN messages m ON m.id = s.msg_id
		WHERE s.user_id = ?
		ORDER BY s.starred_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StarredMessage
	for rows.Next() {
		m := &model.Message{}
		var edited sql.NullTime
		var sm StarredMessage
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt, &edited,
			&sm.StarredAt,
		); err != nil {
			return nil, err
		}
		if edited.Valid {
			t := edited.Time
			m.EditedAt = &t
		}
		sm.Message = m
		out = append(out, sm)
	}
	return out, rows.Err()
}

func (s *MessageStore) ListStarredIDs(ctx context.Context, userID int64, msgIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(msgIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, 0, len(msgIDs)+1)
	args = append(args, userID)
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT msg_id FROM user_starred_messages WHERE user_id = ? AND msg_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

type PollOptionCount struct {
	OptionID string `json:"option_id"`
	Count    int    `json:"count"`
}

type PollResult struct {
	MsgID      int64             `json:"msg_id,string"`
	Total      int               `json:"total"`
	Counts     []PollOptionCount `json:"counts"`
	MyOptionID string            `json:"my_option_id,omitempty"`
}

func (s *MessageStore) UpsertPollVote(ctx context.Context, convID string, msgID, userID int64, optionID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO poll_votes (msg_id, conversation_id, option_id, user_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE option_id = VALUES(option_id), created_at = CURRENT_TIMESTAMP(3)`,
		msgID, convID, optionID, userID)
	return err
}

func (s *MessageStore) PollResults(ctx context.Context, msgID, viewerID int64) (*PollResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT option_id, COUNT(*) FROM poll_votes WHERE msg_id = ? GROUP BY option_id`, msgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &PollResult{MsgID: msgID, Counts: []PollOptionCount{}}
	for rows.Next() {
		var oc PollOptionCount
		if err := rows.Scan(&oc.OptionID, &oc.Count); err != nil {
			return nil, err
		}
		res.Total += oc.Count
		res.Counts = append(res.Counts, oc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var myOpt sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT option_id FROM poll_votes WHERE msg_id = ? AND user_id = ?`, msgID, viewerID).Scan(&myOpt)
	if myOpt.Valid {
		res.MyOptionID = myOpt.String
	}
	return res, nil
}

func (s *MessageStore) PollResultsForMessages(ctx context.Context, viewerID int64, msgIDs []int64) (map[int64]*PollResult, error) {
	out := map[int64]*PollResult{}
	if len(msgIDs) == 0 {
		return out, nil
	}
	for _, id := range msgIDs {
		out[id] = &PollResult{MsgID: id, Counts: []PollOptionCount{}}
	}
	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, len(msgIDs))
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT msg_id, option_id, COUNT(*) FROM poll_votes WHERE msg_id IN (` + strings.Join(placeholders, ",") + `) GROUP BY msg_id, option_id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var msgID int64
		var oc PollOptionCount
		if err := rows.Scan(&msgID, &oc.OptionID, &oc.Count); err != nil {
			return nil, err
		}
		r := out[msgID]
		r.Total += oc.Count
		r.Counts = append(r.Counts, oc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	q2 := `SELECT msg_id, option_id FROM poll_votes WHERE user_id = ? AND msg_id IN (` + strings.Join(placeholders, ",") + `)`
	args2 := append([]interface{}{viewerID}, args...)
	rows2, err := s.db.QueryContext(ctx, q2, args2...)
	if err != nil {
		return out, nil
	}
	defer rows2.Close()
	for rows2.Next() {
		var msgID int64
		var opt string
		if err := rows2.Scan(&msgID, &opt); err != nil {
			return nil, err
		}
		if r := out[msgID]; r != nil {
			r.MyOptionID = opt
		}
	}
	return out, rows2.Err()
}

const (
	RemindPending   int8 = 0
	RemindFired     int8 = 1
	RemindCancelled int8 = 2
)

type MessageReminder struct {
	ID             int64     `json:"id,string"`
	UserID         int64     `json:"user_id,string"`
	ConversationID string    `json:"conversation_id"`
	MsgID          int64     `json:"msg_id,string"`
	Preview        string    `json:"preview"`
	RemindAt       time.Time `json:"remind_at"`
	Status         int8      `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *MessageStore) InsertReminder(ctx context.Context, r *MessageReminder) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO message_reminders
			(user_id, conversation_id, msg_id, preview, remind_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.UserID, r.ConversationID, r.MsgID, r.Preview, r.RemindAt, RemindPending)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	r.ID = id
	r.Status = RemindPending
	return nil
}

func (s *MessageStore) ListReminders(ctx context.Context, userID int64) ([]MessageReminder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, conversation_id, msg_id, preview, remind_at, status, created_at
		FROM message_reminders
		WHERE user_id = ? AND status = ?
		ORDER BY remind_at ASC
		LIMIT 50`, userID, RemindPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageReminder
	for rows.Next() {
		var r MessageReminder
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ConversationID, &r.MsgID, &r.Preview,
			&r.RemindAt, &r.Status, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *MessageStore) CancelReminder(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE message_reminders SET status = ?
		WHERE id = ? AND user_id = ? AND status = ?`,
		RemindCancelled, id, userID, RemindPending)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MessageStore) ClaimDueReminders(ctx context.Context, limit int) ([]MessageReminder, error) {
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, conversation_id, msg_id, preview, remind_at, status, created_at
		FROM message_reminders
		WHERE status = ? AND remind_at <= NOW(3)
		ORDER BY remind_at ASC
		LIMIT ? FOR UPDATE`, RemindPending, limit)
	if err != nil {
		return nil, err
	}
	var out []MessageReminder
	var ids []int64
	for rows.Next() {
		var r MessageReminder
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.ConversationID, &r.MsgID, &r.Preview,
			&r.RemindAt, &r.Status, &r.CreatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
		ids = append(ids, r.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE message_reminders SET status = ? WHERE id = ? AND status = ?`,
			RemindFired, id, RemindPending); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

type HashtagCount struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

func (s *MessageStore) ReplaceHashtags(ctx context.Context, convID string, msgID int64, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_hashtags WHERE msg_id = ?`, msgID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || len([]rune(tag)) > 64 {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO message_hashtags (conversation_id, msg_id, tag)
			VALUES (?, ?, ?)`, convID, msgID, tag); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MessageStore) DeleteHashtagsForMessage(ctx context.Context, msgID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM message_hashtags WHERE msg_id = ?`, msgID)
	return err
}

func (s *MessageStore) ListHashtags(ctx context.Context, convID string, limit int) ([]HashtagCount, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tag, COUNT(*) AS cnt
		FROM message_hashtags
		WHERE conversation_id = ?
		GROUP BY tag
		ORDER BY cnt DESC, tag ASC
		LIMIT ?`, convID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HashtagCount
	for rows.Next() {
		var h HashtagCount
		if err := rows.Scan(&h.Tag, &h.Count); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *MessageStore) ListByHashtag(ctx context.Context, convID, tag string, beforeSeq int64, limit int) ([]*model.Message, error) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `SELECT ` + messageSelectCols + `
		FROM messages m
		INNER JOIN message_hashtags h ON h.msg_id = m.id
		WHERE h.conversation_id = ? AND h.tag = ? AND m.msg_type = ?`
	args := []interface{}{convID, tag, model.MsgTypeText}
	if beforeSeq > 0 {
		q += ` AND m.seq < ?`
		args = append(args, beforeSeq)
	}
	q += ` ORDER BY m.seq DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*model.Message
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}
