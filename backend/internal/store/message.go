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
		SELECT id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at
		FROM messages WHERE conversation_id = ? AND id = ?`, convID, msgID)
	m := &model.Message{}
	err := row.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *MessageStore) Recall(ctx context.Context, convID string, msgID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE messages SET msg_type = ?, content = ?
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

func (s *MessageStore) GetByClientMsgID(ctx context.Context, convID, clientMsgID string) (*model.Message, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at
		FROM messages WHERE conversation_id = ? AND client_msg_id = ?`, convID, clientMsgID)
	m := &model.Message{}
	err := row.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *MessageStore) ListByConversation(ctx context.Context, convID string, beforeSeq int64, limit int) ([]*model.Message, error) {
	q := `SELECT id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at
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
		m := &model.Message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *MessageStore) SearchInConversation(ctx context.Context, convID, keyword string, beforeSeq int64, limit int) ([]*model.Message, error) {
	q := `SELECT id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at
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
		m := &model.Message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt); err != nil {
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
		SELECT id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id, created_at
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
		m := &model.Message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt); err != nil {
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

func (s *MessageStore) ListOfflineMessages(ctx context.Context, userID int64) ([]*model.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at
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
		m := &model.Message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.Seq, &m.MsgType, &m.Content, &m.ClientMsgID, &m.CreatedAt); err != nil {
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
