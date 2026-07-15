package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type FriendStore struct {
	db *sql.DB
}

func NewFriendStore(db *sql.DB) *FriendStore {
	return &FriendStore{db: db}
}

func (s *FriendStore) AreFriends(ctx context.Context, a, b int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM friendships WHERE user_id = ? AND friend_id = ?`, a, b).Scan(&n)
	return n > 0, err
}

func (s *FriendStore) HasPendingRequest(ctx context.Context, from, to int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM friend_requests WHERE from_user_id = ? AND to_user_id = ? AND status = 0`,
		from, to).Scan(&n)
	return n > 0, err
}

func (s *FriendStore) CreateRequest(ctx context.Context, from, to int64, message string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO friend_requests (from_user_id, to_user_id, message, status) VALUES (?, ?, ?, 0)`,
		from, to, message)
	return err
}

func (s *FriendStore) AcceptRequest(ctx context.Context, reqID, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var from, to int64
	err = tx.QueryRowContext(ctx, `
		SELECT from_user_id, to_user_id FROM friend_requests WHERE id = ? AND to_user_id = ? AND status = 0`,
		reqID, userID).Scan(&from, &to)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE friend_requests SET status = 1 WHERE id = ?`, reqID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO friendships (user_id, friend_id) VALUES (?, ?), (?, ?)`,
		from, to, to, from); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *FriendStore) EnsureFriendship(ctx context.Context, a, b int64) error {
	if a == b {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT IGNORE INTO friendships (user_id, friend_id) VALUES (?, ?), (?, ?)`,
		a, b, b, a)
	return err
}

func (s *FriendStore) RejectRequest(ctx context.Context, reqID, userID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE friend_requests SET status = 2 WHERE id = ? AND to_user_id = ? AND status = 0`,
		reqID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) DeleteFriend(ctx context.Context, userID, friendID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM friendships WHERE user_id = ? AND friend_id = ?`, userID, friendID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM friendships WHERE user_id = ? AND friend_id = ?`, friendID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *FriendStore) ListFriends(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT friend_id FROM friendships WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type FriendEntry struct {
	FriendID int64
	Remark   string
}

func (s *FriendStore) ListFriendEntries(ctx context.Context, userID int64) ([]FriendEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT friend_id, remark FROM friendships WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []FriendEntry
	for rows.Next() {
		var e FriendEntry
		if err := rows.Scan(&e.FriendID, &e.Remark); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (s *FriendStore) SetRemark(ctx context.Context, userID, friendID int64, remark string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE friendships SET remark = ? WHERE user_id = ? AND friend_id = ?`, remark, userID, friendID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

const (
	GroupInviteStatusPending  = 0
	GroupInviteStatusAccepted = 1
	GroupInviteStatusRejected = 2
	GroupInviteStatusCancelled = 3

	GroupInviteTypeFriend     = 0
	GroupInviteTypeFaceToFace = 1
	GroupInviteTypeGroupNo    = 2

	GroupRoleMember = 0
	GroupRoleAdmin  = 1
	GroupRoleOwner  = 2
)

func randomFourDigits() string {
	return fmt.Sprintf("%04d", 1000+rand.Intn(9000))
}

func randomGroupNo() string {
	return fmt.Sprintf("%010d", int64(1000000000)+rand.Int63n(9000000000))
}

func (s *FriendStore) allocateGroupNo(ctx context.Context, tx *sql.Tx) (string, error) {
	for i := 0; i < 30; i++ {
		no := randomGroupNo()
		var n int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM `+"`groups`"+` WHERE group_no = ?`, no).Scan(&n)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return no, nil
		}
	}
	return "", errors.New("无法分配群号")
}

func (s *FriendStore) CreateGroup(ctx context.Context, groupID, ownerID int64, name string) (convID, groupNo string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	groupNo, err = s.allocateGroupNo(ctx, tx)
	if err != nil {
		return "", "", err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO `groups` (id, name, group_no, owner_id) VALUES (?, ?, ?, ?)", groupID, name, groupNo, ownerID); err != nil {
		return "", "", err
	}
	convID = "g_" + idformat(groupID)
	if _, err = tx.ExecContext(ctx, `INSERT INTO conversations (id, type) VALUES (?, 2)`, convID); err != nil {
		return "", "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES (?, ?, ?)`, groupID, ownerID, GroupRoleOwner); err != nil {
		return "", "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO conversation_members (conversation_id, user_id) VALUES (?, ?)`, convID, ownerID); err != nil {
		return "", "", err
	}
	return convID, groupNo, tx.Commit()
}

func (s *FriendStore) ListGroupMemberIDs(ctx context.Context, groupID int64) ([]int64, error) {
	roles, err := s.ListGroupMemberRoles(ctx, groupID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(roles))
	for i, r := range roles {
		ids[i] = r.UserID
	}
	return ids, nil
}

type GroupMemberRole struct {
	UserID   int64
	Role     int
	Muted    bool
	Nickname string
}

func (s *FriendStore) ListGroupMemberRoles(ctx context.Context, groupID int64) ([]GroupMemberRole, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, role, COALESCE(muted, 0), COALESCE(nickname, '')
		FROM group_members WHERE group_id = ? ORDER BY role DESC, user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupMemberRole
	for rows.Next() {
		var r GroupMemberRole
		var muted int
		if err := rows.Scan(&r.UserID, &r.Role, &muted, &r.Nickname); err != nil {
			return nil, err
		}
		r.Muted = muted == 1
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *FriendStore) ListPendingRequests(ctx context.Context, userID int64) ([]struct {
	ID         int64
	FromUserID int64
	Message    string
}, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, from_user_id, message FROM friend_requests WHERE to_user_id = ? AND status = 0`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []struct {
		ID         int64
		FromUserID int64
		Message    string
	}
	for rows.Next() {
		var r struct {
			ID         int64
			FromUserID int64
			Message    string
		}
		if err := rows.Scan(&r.ID, &r.FromUserID, &r.Message); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

type GroupSummary struct {
	ID             int64  `json:"id,string"`
	Name           string `json:"name"`
	GroupNo        string `json:"group_no"`
	OwnerID        int64  `json:"owner_id,string"`
	ConversationID string `json:"conversation_id"`
	Notice         string `json:"notice"`
	WelcomeText    string `json:"welcome_text"`
	AdminOnly      bool   `json:"admin_only"`
	SlowModeSecs   int    `json:"slow_mode_secs"`
}

type GroupPublicSummary struct {
	ID          int64  `json:"id,string"`
	Name        string `json:"name"`
	GroupNo     string `json:"group_no"`
	MemberCount int    `json:"member_count"`
}

type GroupInvitation struct {
	ID         int64
	GroupID    int64
	FromUserID int64
	ToUserID   int64
	Message    string
	InviteType int
	Status     int
	CreatedAt  time.Time
}

type FaceSession struct {
	ID        int64
	GroupID   int64
	Code      string
	CreatedBy int64
	ExpiresAt time.Time
}

func (s *FriendStore) ListUserGroups(ctx context.Context, userID int64) ([]GroupSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.group_no, g.owner_id, g.notice, COALESCE(g.welcome_text, ''), COALESCE(g.admin_only, 0), COALESCE(g.slow_mode_secs, 0)
		FROM `+"`groups`"+` g
		INNER JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupSummary
	for rows.Next() {
		var g GroupSummary
		var adminOnly int
		if err := rows.Scan(&g.ID, &g.Name, &g.GroupNo, &g.OwnerID, &g.Notice, &g.WelcomeText, &adminOnly, &g.SlowModeSecs); err != nil {
			return nil, err
		}
		g.AdminOnly = adminOnly == 1
		g.ConversationID = "g_" + idformat(g.ID)
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *FriendStore) SearchGroups(ctx context.Context, userID int64, q string, limit int) ([]GroupSummary, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	like := "%" + q + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.group_no, g.owner_id, g.notice, COALESCE(g.welcome_text, ''), COALESCE(g.admin_only, 0), COALESCE(g.slow_mode_secs, 0)
		FROM `+"`groups`"+` g
		INNER JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = ?
		WHERE g.name LIKE ? OR g.group_no LIKE ? LIMIT ?`, userID, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupSummary
	for rows.Next() {
		var g GroupSummary
		var adminOnly int
		if err := rows.Scan(&g.ID, &g.Name, &g.GroupNo, &g.OwnerID, &g.Notice, &g.WelcomeText, &adminOnly, &g.SlowModeSecs); err != nil {
			return nil, err
		}
		g.AdminOnly = adminOnly == 1
		g.ConversationID = "g_" + idformat(g.ID)
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *FriendStore) GetGroup(ctx context.Context, groupID int64) (*GroupSummary, []int64, error) {
	var g GroupSummary
	var adminOnly int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, group_no, owner_id, notice, COALESCE(welcome_text, ''), COALESCE(admin_only, 0), COALESCE(slow_mode_secs, 0)
		FROM `+"`groups`"+` WHERE id = ?`, groupID).
		Scan(&g.ID, &g.Name, &g.GroupNo, &g.OwnerID, &g.Notice, &g.WelcomeText, &adminOnly, &g.SlowModeSecs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	g.AdminOnly = adminOnly == 1
	g.ConversationID = "g_" + idformat(g.ID)
	if g.GroupNo == "" {
		if no, err := s.EnsureGroupNo(ctx, groupID); err == nil {
			g.GroupNo = no
		}
	}
	members, err := s.ListGroupMemberIDs(ctx, groupID)
	return &g, members, err
}

func (s *FriendStore) EnsureGroupNo(ctx context.Context, groupID int64) (string, error) {
	var existing sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT group_no FROM `+"`groups`"+` WHERE id = ?`, groupID).Scan(&existing)
	if err != nil {
		return "", err
	}
	if existing.Valid && existing.String != "" {
		return existing.String, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	no, err := s.allocateGroupNo(ctx, tx)
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE `+"`groups`"+` SET group_no = ? WHERE id = ?`, no, groupID); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return no, nil
}

func (s *FriendStore) SetGroupNotice(ctx context.Context, groupID int64, notice string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE `+"`groups`"+` SET notice = ? WHERE id = ?`, notice, groupID)
	return err
}

func (s *FriendStore) SetGroupWelcome(ctx context.Context, groupID int64, welcome string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE `+"`groups`"+` SET welcome_text = ? WHERE id = ?`, welcome, groupID)
	return err
}

func (s *FriendStore) SetGroupAdminOnly(ctx context.Context, groupID int64, adminOnly bool) error {
	v := 0
	if adminOnly {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE `+"`groups`"+` SET admin_only = ? WHERE id = ?`, v, groupID)
	return err
}

func (s *FriendStore) SetGroupSlowMode(ctx context.Context, groupID int64, secs int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE `+"`groups`"+` SET slow_mode_secs = ? WHERE id = ?`, secs, groupID)
	return err
}

func (s *FriendStore) SetMemberMuted(ctx context.Context, groupID, userID int64, muted bool) error {
	v := 0
	if muted {
		v = 1
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE group_members SET muted = ? WHERE group_id = ? AND user_id = ?`, v, groupID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) SetMemberNickname(ctx context.Context, groupID, userID int64, nickname string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE group_members SET nickname = ? WHERE group_id = ? AND user_id = ?`, nickname, groupID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) IsMemberMuted(ctx context.Context, groupID, userID int64) (bool, error) {
	var muted int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(muted, 0) FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&muted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return muted == 1, err
}

func (s *FriendStore) IsGroupMember(ctx context.Context, groupID, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&n)
	return n > 0, err
}

func (s *FriendStore) GetMemberRole(ctx context.Context, groupID, userID int64) (int, error) {
	var role int
	err := s.db.QueryRowContext(ctx, `SELECT role FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return role, err
}

func (s *FriendStore) CountGroupMembers(ctx context.Context, groupID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM group_members WHERE group_id = ?`, groupID).Scan(&n)
	return n, err
}

func (s *FriendStore) GetGroupByNo(ctx context.Context, groupNo string) (*GroupSummary, error) {
	var g GroupSummary
	var adminOnly int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, group_no, owner_id, notice, COALESCE(welcome_text, ''), COALESCE(admin_only, 0), COALESCE(slow_mode_secs, 0)
		FROM `+"`groups`"+` WHERE group_no = ?`, groupNo).
		Scan(&g.ID, &g.Name, &g.GroupNo, &g.OwnerID, &g.Notice, &g.WelcomeText, &adminOnly, &g.SlowModeSecs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.AdminOnly = adminOnly == 1
	g.ConversationID = "g_" + idformat(g.ID)
	return &g, nil
}

func (s *FriendStore) SearchGroupsByNo(ctx context.Context, q string, limit int) ([]GroupPublicSummary, error) {
	return s.DiscoverGroups(ctx, q, limit)
}

func (s *FriendStore) DiscoverGroups(ctx context.Context, q string, limit int) ([]GroupPublicSummary, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	allDigits := true
	for _, c := range q {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	var rows *sql.Rows
	var err error
	if allDigits && len(q) >= 6 {
		like := q + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT g.id, g.name, COALESCE(g.group_no, ''), (SELECT COUNT(1) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
			FROM `+"`groups`"+` g
			WHERE g.group_no LIKE ?
			ORDER BY g.group_no
			LIMIT ?`, like, limit)
	} else {
		like := "%" + q + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT g.id, g.name, COALESCE(g.group_no, ''), (SELECT COUNT(1) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
			FROM `+"`groups`"+` g
			WHERE g.name LIKE ?
			ORDER BY g.name
			LIMIT ?`, like, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupPublicSummary
	for rows.Next() {
		var g GroupPublicSummary
		if err := rows.Scan(&g.ID, &g.Name, &g.GroupNo, &g.MemberCount); err != nil {
			return nil, err
		}
		if g.GroupNo == "" {
			if no, err := s.EnsureGroupNo(ctx, g.ID); err == nil {
				g.GroupNo = no
			}
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *FriendStore) IsActiveFaceCodeTaken(ctx context.Context, code string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM group_face_sessions WHERE code = ? AND expires_at > ?`, code, time.Now()).Scan(&n)
	return n > 0, err
}

func (s *FriendStore) CreateFaceSessionWithCode(ctx context.Context, groupID, createdBy int64, code string, expiresAt time.Time) error {
	taken, err := s.IsActiveFaceCodeTaken(ctx, code)
	if err != nil {
		return err
	}
	if taken {
		return errors.New("face code taken")
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO group_face_sessions (group_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)`,
		groupID, code, createdBy, expiresAt)
	return err
}

func (s *FriendStore) AddGroupMember(ctx context.Context, groupID, userID int64) error {
	return s.AddGroupMembers(ctx, groupID, []int64{userID})
}

func (s *FriendStore) HasPendingGroupInvite(ctx context.Context, groupID, toUserID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM group_invitations WHERE group_id = ? AND to_user_id = ? AND status = 0`,
		groupID, toUserID).Scan(&n)
	return n > 0, err
}

func (s *FriendStore) CreateGroupInvitation(ctx context.Context, groupID, fromUserID, toUserID int64, message string, inviteType int) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO group_invitations (group_id, from_user_id, to_user_id, message, invite_type, status)
		VALUES (?, ?, ?, ?, ?, 0)`,
		groupID, fromUserID, toUserID, message, inviteType)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *FriendStore) ListPendingGroupInvites(ctx context.Context, toUserID int64) ([]GroupInvitation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, from_user_id, to_user_id, message, invite_type, status, created_at
		FROM group_invitations WHERE to_user_id = ? AND status = 0 ORDER BY id DESC`, toUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupInvitation
	for rows.Next() {
		var inv GroupInvitation
		if err := rows.Scan(&inv.ID, &inv.GroupID, &inv.FromUserID, &inv.ToUserID, &inv.Message, &inv.InviteType, &inv.Status, &inv.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (s *FriendStore) AcceptGroupInvitation(ctx context.Context, inviteID, userID int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var inv GroupInvitation
	err = tx.QueryRowContext(ctx, `
		SELECT id, group_id, from_user_id, to_user_id, message, invite_type, status
		FROM group_invitations WHERE id = ? AND to_user_id = ? AND status = 0`,
		inviteID, userID).Scan(&inv.ID, &inv.GroupID, &inv.FromUserID, &inv.ToUserID, &inv.Message, &inv.InviteType, &inv.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE group_invitations SET status = ? WHERE id = ?`, GroupInviteStatusAccepted, inviteID); err != nil {
		return 0, err
	}
	convID := "g_" + idformat(inv.GroupID)
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id, role) VALUES (?, ?, ?)`, inv.GroupID, userID, GroupRoleMember); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO conversation_members (conversation_id, user_id) VALUES (?, ?)`, convID, userID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return inv.GroupID, nil
}

func (s *FriendStore) RejectGroupInvitation(ctx context.Context, inviteID, userID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE group_invitations SET status = ? WHERE id = ? AND to_user_id = ? AND status = 0`,
		GroupInviteStatusRejected, inviteID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) ListPendingGroupInvitesByGroup(ctx context.Context, groupID int64) ([]GroupInvitation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, from_user_id, to_user_id, message, invite_type, status, created_at
		FROM group_invitations WHERE group_id = ? AND status = 0 ORDER BY id DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupInvitation
	for rows.Next() {
		var inv GroupInvitation
		if err := rows.Scan(&inv.ID, &inv.GroupID, &inv.FromUserID, &inv.ToUserID, &inv.Message, &inv.InviteType, &inv.Status, &inv.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (s *FriendStore) CancelGroupInvitation(ctx context.Context, inviteID, groupID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE group_invitations SET status = ? WHERE id = ? AND group_id = ? AND status = 0`,
		GroupInviteStatusCancelled, inviteID, groupID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) TransferGroupOwner(ctx context.Context, groupID, newOwnerID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldOwner int64
	err = tx.QueryRowContext(ctx, `SELECT owner_id FROM `+"`groups`"+` WHERE id = ?`, groupID).Scan(&oldOwner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE `+"`groups`"+` SET owner_id = ? WHERE id = ?`, newOwnerID, groupID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE group_members SET role = ? WHERE group_id = ? AND user_id = ?`, GroupRoleOwner, groupID, newOwnerID); err != nil {
		return err
	}
	if oldOwner != newOwnerID {
		if _, err = tx.ExecContext(ctx, `UPDATE group_members SET role = ? WHERE group_id = ? AND user_id = ?`, GroupRoleMember, groupID, oldOwner); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *FriendStore) allocateFaceCode(ctx context.Context, tx *sql.Tx) (string, error) {
	now := time.Now()
	for i := 0; i < 30; i++ {
		code := randomFourDigits()
		var n int
		err := tx.QueryRowContext(ctx, `
			SELECT COUNT(1) FROM group_face_sessions WHERE code = ? AND expires_at > ?`, code, now).Scan(&n)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return code, nil
		}
	}
	return "", errors.New("无法分配面对面建群码")
}

func (s *FriendStore) CreateFaceSession(ctx context.Context, groupID, createdBy int64, expiresAt time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	code, err := s.allocateFaceCode(ctx, tx)
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO group_face_sessions (group_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)`,
		groupID, code, createdBy, expiresAt); err != nil {
		return "", err
	}
	return code, tx.Commit()
}

func (s *FriendStore) GetActiveFaceSessionByCode(ctx context.Context, code string) (*FaceSession, error) {
	var sess FaceSession
	err := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, code, created_by, expires_at
		FROM group_face_sessions WHERE code = ? AND expires_at > ? ORDER BY id DESC LIMIT 1`,
		code, time.Now()).Scan(&sess.ID, &sess.GroupID, &sess.Code, &sess.CreatedBy, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *FriendStore) SetMemberRole(ctx context.Context, groupID, userID int64, role int) error {
	res, err := s.db.ExecContext(ctx, `UPDATE group_members SET role = ? WHERE group_id = ? AND user_id = ?`, role, groupID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *FriendStore) RefreshFaceSession(ctx context.Context, groupID, createdBy int64, expiresAt time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM group_face_sessions WHERE group_id = ?`, groupID); err != nil {
		return "", err
	}
	code, err := s.allocateFaceCode(ctx, tx)
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO group_face_sessions (group_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)`,
		groupID, code, createdBy, expiresAt); err != nil {
		return "", err
	}
	return code, tx.Commit()
}

func (s *FriendStore) GetActiveFaceSessionByGroup(ctx context.Context, groupID int64) (*FaceSession, error) {
	var sess FaceSession
	err := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, code, created_by, expires_at
		FROM group_face_sessions WHERE group_id = ? AND expires_at > ? ORDER BY id DESC LIMIT 1`,
		groupID, time.Now()).Scan(&sess.ID, &sess.GroupID, &sess.Code, &sess.CreatedBy, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *FriendStore) AddGroupMembers(ctx context.Context, groupID int64, userIDs []int64) error {
	convID := "g_" + idformat(groupID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, uid := range userIDs {
		if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id, role) VALUES (?, ?, 0)`, groupID, uid); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO conversation_members (conversation_id, user_id) VALUES (?, ?)`, convID, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *FriendStore) RemoveGroupMember(ctx context.Context, groupID, userID int64) error {
	convID := "g_" + idformat(groupID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?`, convID, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func idformat(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

type GroupInviteLink struct {
	ID        int64
	GroupID   int64
	Code      string
	CreatedBy int64
	MaxUses   int
	UseCount  int
	ExpiresAt *time.Time
	Revoked   bool
	CreatedAt time.Time
}

func (s *FriendStore) CreateInviteLink(ctx context.Context, link *GroupInviteLink) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO group_invite_links (id, group_id, code, created_by, max_uses, use_count, expires_at, revoked)
		VALUES (?, ?, ?, ?, ?, 0, ?, 0)`,
		link.ID, link.GroupID, link.Code, link.CreatedBy, link.MaxUses, link.ExpiresAt)
	return err
}

func (s *FriendStore) ListInviteLinks(ctx context.Context, groupID int64) ([]GroupInviteLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, code, created_by, max_uses, use_count, expires_at, revoked, created_at
		FROM group_invite_links WHERE group_id = ? AND revoked = 0 ORDER BY id DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []GroupInviteLink
	for rows.Next() {
		var l GroupInviteLink
		var revoked int
		var exp sql.NullTime
		if err := rows.Scan(&l.ID, &l.GroupID, &l.Code, &l.CreatedBy, &l.MaxUses, &l.UseCount, &exp, &revoked, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Revoked = revoked == 1
		if exp.Valid {
			t := exp.Time
			l.ExpiresAt = &t
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (s *FriendStore) GetInviteLinkByCode(ctx context.Context, code string) (*GroupInviteLink, error) {
	var l GroupInviteLink
	var revoked int
	var exp sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, code, created_by, max_uses, use_count, expires_at, revoked, created_at
		FROM group_invite_links WHERE code = ?`, code).Scan(
		&l.ID, &l.GroupID, &l.Code, &l.CreatedBy, &l.MaxUses, &l.UseCount, &exp, &revoked, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	l.Revoked = revoked == 1
	if exp.Valid {
		t := exp.Time
		l.ExpiresAt = &t
	}
	return &l, nil
}

func (s *FriendStore) RevokeInviteLink(ctx context.Context, groupID, linkID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE group_invite_links SET revoked = 1 WHERE id = ? AND group_id = ? AND revoked = 0`, linkID, groupID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeInviteLink atomically increments use_count when the link is still valid.
func (s *FriendStore) ConsumeInviteLink(ctx context.Context, code string) (*GroupInviteLink, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var l GroupInviteLink
	var revoked int
	var exp sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id, group_id, code, created_by, max_uses, use_count, expires_at, revoked, created_at
		FROM group_invite_links WHERE code = ? FOR UPDATE`, code).Scan(
		&l.ID, &l.GroupID, &l.Code, &l.CreatedBy, &l.MaxUses, &l.UseCount, &exp, &revoked, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	l.Revoked = revoked == 1
	if exp.Valid {
		t := exp.Time
		l.ExpiresAt = &t
	}
	if l.Revoked {
		return nil, errors.New("link revoked")
	}
	if l.ExpiresAt != nil && !l.ExpiresAt.After(time.Now()) {
		return nil, errors.New("link expired")
	}
	if l.MaxUses > 0 && l.UseCount >= l.MaxUses {
		return nil, errors.New("link exhausted")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE group_invite_links SET use_count = use_count + 1 WHERE id = ?`, l.ID); err != nil {
		return nil, err
	}
	l.UseCount++
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *FriendStore) SetGroupMemberRemark(ctx context.Context, viewerID, groupID, targetID int64, remark string) error {
	if remark == "" {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM group_member_remarks WHERE user_id = ? AND group_id = ? AND target_user_id = ?`,
			viewerID, groupID, targetID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO group_member_remarks (user_id, group_id, target_user_id, remark)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE remark = VALUES(remark)`,
		viewerID, groupID, targetID, remark)
	return err
}

func (s *FriendStore) ListGroupMemberRemarks(ctx context.Context, viewerID, groupID int64) (map[int64]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT target_user_id, remark FROM group_member_remarks
		WHERE user_id = ? AND group_id = ? AND remark <> ''`, viewerID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var tid int64
		var remark string
		if err := rows.Scan(&tid, &remark); err != nil {
			return nil, err
		}
		out[tid] = remark
	}
	return out, rows.Err()
}
