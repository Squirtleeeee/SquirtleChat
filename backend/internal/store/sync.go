package store

import (
	"context"
	"database/sql"

	"squirtlechat/internal/model"
)

type SyncStore struct {
	db *sql.DB
}

func NewSyncStore(db *sql.DB) *SyncStore {
	return &SyncStore{db: db}
}

func (s *SyncStore) GetCursor(ctx context.Context, userID int64, deviceID string) (int64, error) {
	var seq int64
	err := s.db.QueryRowContext(ctx, `
		SELECT last_sync_seq FROM user_devices WHERE user_id = ? AND device_id = ?`, userID, deviceID).Scan(&seq)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return seq, err
}

func (s *SyncStore) SetCursor(ctx context.Context, userID int64, deviceID string, seq int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_devices SET last_sync_seq = ? WHERE user_id = ? AND device_id = ?`, seq, userID, deviceID)
	return err
}

func (s *SyncStore) ListMessagesSince(ctx context.Context, userID, sinceSeq int64, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.from_user_id, m.seq, m.msg_type, m.content, m.client_msg_id, m.created_at
		FROM messages m
		INNER JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = ?
		WHERE m.id > ?
		ORDER BY m.id ASC
		LIMIT ?`, userID, sinceSeq, limit)
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

func (s *SyncStore) MarkRead(ctx context.Context, userID int64, convID string, readSeq int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE conversation_members SET last_read_seq = GREATEST(last_read_seq, ?)
		WHERE conversation_id = ? AND user_id = ?`, readSeq, convID, userID)
	return err
}
