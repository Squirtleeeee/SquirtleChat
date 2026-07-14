package service

import (
	"context"
	"strconv"

	"squirtlechat/internal/model"
	"squirtlechat/internal/store"
)

type SyncService struct {
	sync   *store.SyncStore
	msgs   *store.MessageStore
	users  *store.UserStore
	onRead func(ctx context.Context, readerID int64, convID string, readSeq int64)
}

func NewSyncService(sync *store.SyncStore, msgs *store.MessageStore, users *store.UserStore) *SyncService {
	return &SyncService{sync: sync, msgs: msgs, users: users}
}

func (s *SyncService) SetOnRead(fn func(ctx context.Context, readerID int64, convID string, readSeq int64)) {
	s.onRead = fn
}

type SyncResult struct {
	Messages   []*model.Message `json:"messages"`
	NextCursor int64            `json:"next_cursor"`
}

func (s *SyncService) Sync(ctx context.Context, userID int64, deviceID string, sinceSeq int64, limit int) (*SyncResult, error) {
	if sinceSeq == 0 {
		cur, _ := s.sync.GetCursor(ctx, userID, deviceID)
		sinceSeq = cur
	}
	list, err := s.sync.ListMessagesSince(ctx, userID, sinceSeq, limit)
	if err != nil {
		return nil, err
	}
	offline, err := s.msgs.ListOfflineMessages(ctx, userID)
	if err == nil && len(offline) > 0 {
		seen := make(map[int64]bool, len(list))
		for _, m := range list {
			seen[m.ID] = true
		}
		var delivered []int64
		for _, m := range offline {
			if !seen[m.ID] {
				list = append(list, m)
				seen[m.ID] = true
			}
			delivered = append(delivered, m.ID)
		}
		_ = s.msgs.ClearOffline(ctx, userID, delivered)
	}
	next := sinceSeq
	for _, m := range list {
		if m.ID > next {
			next = m.ID
		}
	}
	if deviceID != "" && next > sinceSeq {
		_ = s.sync.SetCursor(ctx, userID, deviceID, next)
	}
	return &SyncResult{Messages: list, NextCursor: next}, nil
}

func (s *SyncService) MarkRead(ctx context.Context, userID int64, convID string, readSeq int64) error {
	if err := s.sync.MarkRead(ctx, userID, convID, readSeq); err != nil {
		return err
	}
	if s.onRead != nil {
		s.onRead(ctx, userID, convID, readSeq)
	}
	return nil
}

func (s *SyncService) GetReadState(ctx context.Context, viewerID int64, convID string) ([]map[string]interface{}, error) {
	ok, err := s.msgs.IsConversationMember(ctx, convID, viewerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errBadRequest("您不在该会话中")
	}
	rows, err := s.msgs.ListMemberReadSeqs(ctx, convID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"user_id":  strconv.FormatInt(r.UserID, 10),
			"read_seq": r.ReadSeq,
		})
	}
	return out, nil
}
