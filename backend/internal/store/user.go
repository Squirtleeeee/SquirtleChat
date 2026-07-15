package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"squirtlechat/internal/model"
)

var ErrNotFound = errors.New("记录不存在")
var ErrDuplicate = errors.New("数据已存在")

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(ctx context.Context, u *model.User) error {
	priv, _ := json.Marshal(model.DefaultPrivacy())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, nickname, privacy_json) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Nickname, priv,
	)
	return err
}

const userSelectCols = `id, username, password_hash, nickname, avatar, status_text, status_emoji, gender, birthday, privacy_json, created_at`

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userSelectCols+` FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userSelectCols+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

type searchCandidate struct {
	user  *model.User
	score int
}

func (s *UserStore) Search(ctx context.Context, q string, limit int) ([]*model.User, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	var exact []*model.User
	if id, err := strconv.ParseInt(q, 10, 64); err == nil && id > 0 {
		if u, err := s.GetByID(ctx, id); err == nil {
			exact = append(exact, u)
		}
	}
	if u, err := s.GetByUsername(ctx, q); err == nil {
		exact = append(exact, u)
	}
	if len(exact) > 0 {
		seen := map[int64]bool{}
		out := make([]*model.User, 0, len(exact))
		for _, u := range exact {
			if seen[u.ID] {
				continue
			}
			seen[u.ID] = true
			out = append(out, u)
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return out, nil
	}

	like := "%" + q + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userSelectCols+` FROM users WHERE username LIKE ? OR nickname LIKE ? OR CAST(id AS CHAR) LIKE ? LIMIT 50`,
		like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qLower := strings.ToLower(q)
	var candidates []searchCandidate
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		score := searchScore(qLower, u)
		candidates = append(candidates, searchCandidate{user: u, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		return candidates[i].user.Username < candidates[j].user.Username
	})

	out := make([]*model.User, 0, limit)
	for _, c := range candidates {
		out = append(out, c.user)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func searchScore(qLower string, u *model.User) int {
	best := 1000
	for _, field := range []string{strings.ToLower(u.Username), strings.ToLower(u.Nickname), strconv.FormatInt(u.ID, 10)} {
		if field == "" {
			continue
		}
		if field == qLower {
			return 0
		}
		if strings.HasPrefix(field, qLower) {
			best = min(best, 10+utf8.RuneCountInString(field)-utf8.RuneCountInString(qLower))
			continue
		}
		if strings.Contains(field, qLower) {
			best = min(best, 50+utf8.RuneCountInString(field))
			continue
		}
		d := levenshtein(qLower, field)
		if d <= 3 {
			best = min(best, 100+d*10)
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

type ProfilePatch struct {
	Nickname    *string
	Avatar      *string
	StatusText  *string
	StatusEmoji *string
	Gender      *int8
	Birthday    *string
}

func (s *UserStore) UpdateProfile(ctx context.Context, userID int64, patch ProfilePatch) error {
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	nickname := u.Nickname
	avatar := u.Avatar
	statusText := u.StatusText
	statusEmoji := u.StatusEmoji
	gender := u.Gender
	var birthday *string = u.Birthday
	if patch.Nickname != nil {
		nickname = *patch.Nickname
	}
	if patch.Avatar != nil {
		avatar = *patch.Avatar
	}
	if patch.StatusText != nil {
		statusText = *patch.StatusText
	}
	if patch.StatusEmoji != nil {
		statusEmoji = *patch.StatusEmoji
	}
	if patch.Gender != nil {
		gender = *patch.Gender
	}
	if patch.Birthday != nil {
		if *patch.Birthday == "" {
			birthday = nil
		} else {
			birthday = patch.Birthday
		}
	}
	var bdayVal interface{}
	if birthday != nil && *birthday != "" {
		bdayVal = *birthday
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET nickname = ?, avatar = ?, status_text = ?, status_emoji = ?, gender = ?, birthday = ?, updated_at = NOW(3)
		WHERE id = ?`, nickname, avatar, statusText, statusEmoji, gender, bdayVal, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *UserStore) UpdatePasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = NOW(3) WHERE id = ?`,
		passwordHash, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *UserStore) UpdatePrivacy(ctx context.Context, userID int64, p model.UserPrivacy) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET privacy_json = ?, updated_at = NOW(3) WHERE id = ?`, raw, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *UserStore) UpsertDevice(ctx context.Context, userID int64, deviceID, deviceName string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_devices (user_id, device_id, device_name, last_active_at)
		VALUES (?, ?, ?, NOW(3))
		ON DUPLICATE KEY UPDATE
			device_name = IF(VALUES(device_name) = '', device_name, VALUES(device_name)),
			last_active_at = NOW(3)`,
		userID, deviceID, deviceName)
	return err
}

type UserDevice struct {
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	LastSyncSeq  int64     `json:"last_sync_seq"`
	LastActiveAt time.Time `json:"last_active_at"`
}

func (s *UserStore) ListDevices(ctx context.Context, userID int64) ([]UserDevice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT device_id, device_name, last_sync_seq, last_active_at
		FROM user_devices WHERE user_id = ?
		ORDER BY last_active_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserDevice
	for rows.Next() {
		var d UserDevice
		if err := rows.Scan(&d.DeviceID, &d.DeviceName, &d.LastSyncSeq, &d.LastActiveAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *UserStore) DeleteDevice(ctx context.Context, userID int64, deviceID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM user_devices WHERE user_id = ? AND device_id = ?`, userID, deviceID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type ChatFolder struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ConversationIDs []string `json:"conversation_ids"`
}

type NotifyPrefs struct {
	DesktopEnabled    bool   `json:"desktop_enabled"`
	QuietHoursEnabled bool   `json:"quiet_hours_enabled"`
	QuietStart        string `json:"quiet_start"`
	QuietEnd          string `json:"quiet_end"`
}

type ChatPrefs struct {
	Muted         []string     `json:"muted"`
	PinnedFriends []string     `json:"pinned_friends"`
	PinnedGroups  []string     `json:"pinned_groups"`
	Folders       []ChatFolder `json:"folders"`
	Notify        *NotifyPrefs `json:"notify,omitempty"`
}

func (s *UserStore) GetChatPrefs(ctx context.Context, userID int64) (*ChatPrefs, error) {
	var mutedRaw, friendsRaw, groupsRaw, foldersRaw, notifyRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT muted_json, pinned_friends_json, pinned_groups_json,
			COALESCE(folders_json, JSON_ARRAY()), notify_json
		FROM user_chat_prefs WHERE user_id = ?`, userID).Scan(&mutedRaw, &friendsRaw, &groupsRaw, &foldersRaw, &notifyRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return &ChatPrefs{
			Muted: []string{}, PinnedFriends: []string{}, PinnedGroups: []string{}, Folders: []ChatFolder{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	prefs := &ChatPrefs{}
	_ = json.Unmarshal(mutedRaw, &prefs.Muted)
	_ = json.Unmarshal(friendsRaw, &prefs.PinnedFriends)
	_ = json.Unmarshal(groupsRaw, &prefs.PinnedGroups)
	_ = json.Unmarshal(foldersRaw, &prefs.Folders)
	if len(notifyRaw) > 0 && string(notifyRaw) != "null" {
		var n NotifyPrefs
		if json.Unmarshal(notifyRaw, &n) == nil {
			prefs.Notify = &n
		}
	}
	if prefs.Muted == nil {
		prefs.Muted = []string{}
	}
	if prefs.PinnedFriends == nil {
		prefs.PinnedFriends = []string{}
	}
	if prefs.PinnedGroups == nil {
		prefs.PinnedGroups = []string{}
	}
	if prefs.Folders == nil {
		prefs.Folders = []ChatFolder{}
	}
	for i := range prefs.Folders {
		if prefs.Folders[i].ConversationIDs == nil {
			prefs.Folders[i].ConversationIDs = []string{}
		}
	}
	return prefs, nil
}

func (s *UserStore) UpsertChatPrefs(ctx context.Context, userID int64, prefs ChatPrefs) error {
	if prefs.Muted == nil {
		prefs.Muted = []string{}
	}
	if prefs.PinnedFriends == nil {
		prefs.PinnedFriends = []string{}
	}
	if prefs.PinnedGroups == nil {
		prefs.PinnedGroups = []string{}
	}
	if prefs.Folders == nil {
		prefs.Folders = []ChatFolder{}
	}
	for i := range prefs.Folders {
		if prefs.Folders[i].ConversationIDs == nil {
			prefs.Folders[i].ConversationIDs = []string{}
		}
		prefs.Folders[i].Name = strings.TrimSpace(prefs.Folders[i].Name)
		if prefs.Folders[i].Name == "" {
			prefs.Folders[i].Name = "未命名"
		}
		if len([]rune(prefs.Folders[i].Name)) > 24 {
			prefs.Folders[i].Name = string([]rune(prefs.Folders[i].Name)[:24])
		}
		if len(prefs.Folders[i].ConversationIDs) > 200 {
			prefs.Folders[i].ConversationIDs = prefs.Folders[i].ConversationIDs[:200]
		}
	}
	if len(prefs.Folders) > 20 {
		prefs.Folders = prefs.Folders[:20]
	}
	if prefs.Notify != nil {
		if prefs.Notify.QuietStart == "" {
			prefs.Notify.QuietStart = "22:00"
		}
		if prefs.Notify.QuietEnd == "" {
			prefs.Notify.QuietEnd = "08:00"
		}
	}
	muted, _ := json.Marshal(prefs.Muted)
	friends, _ := json.Marshal(prefs.PinnedFriends)
	groups, _ := json.Marshal(prefs.PinnedGroups)
	folders, _ := json.Marshal(prefs.Folders)
	var notify interface{}
	if prefs.Notify != nil {
		notify, _ = json.Marshal(prefs.Notify)
	} else {
		notify = nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_chat_prefs (user_id, muted_json, pinned_friends_json, pinned_groups_json, folders_json, notify_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			muted_json = VALUES(muted_json),
			pinned_friends_json = VALUES(pinned_friends_json),
			pinned_groups_json = VALUES(pinned_groups_json),
			folders_json = VALUES(folders_json),
			notify_json = COALESCE(VALUES(notify_json), notify_json),
			updated_at = NOW(3)`,
		userID, muted, friends, groups, folders, notify)
	return err
}

const MaxDraftLen = 4000

type DraftItem struct {
	ConversationID string    `json:"conversation_id"`
	Content        string    `json:"content"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *UserStore) ListDrafts(ctx context.Context, userID int64) (map[string]string, error) {
	items, err := s.ListDraftItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, it := range items {
		out[it.ConversationID] = it.Content
	}
	return out, nil
}

func (s *UserStore) ListDraftItems(ctx context.Context, userID int64) ([]DraftItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT conversation_id, content, updated_at FROM user_drafts WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftItem
	for rows.Next() {
		var it DraftItem
		if err := rows.Scan(&it.ConversationID, &it.Content, &it.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *UserStore) UpsertDraft(ctx context.Context, userID int64, convID, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM user_drafts WHERE user_id = ? AND conversation_id = ?`, userID, convID)
		return err
	}
	runes := []rune(content)
	if len(runes) > MaxDraftLen {
		content = string(runes[:MaxDraftLen])
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_drafts (user_id, conversation_id, content)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE content = VALUES(content), updated_at = NOW(3)`,
		userID, convID, content)
	return err
}

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	var gender sql.NullInt64
	var birthday sql.NullTime
	var privacyRaw []byte
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.Avatar, &u.StatusText, &u.StatusEmoji, &gender, &birthday, &privacyRaw, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if gender.Valid {
		u.Gender = int8(gender.Int64)
	}
	if birthday.Valid {
		s := birthday.Time.Format("2006-01-02")
		u.Birthday = &s
	}
	u.Privacy = model.ParsePrivacyJSON(privacyRaw)
	return u, nil
}

func scanUserRow(rows *sql.Rows) (*model.User, error) {
	u := &model.User{}
	var gender sql.NullInt64
	var birthday sql.NullTime
	var privacyRaw []byte
	err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.Avatar, &u.StatusText, &u.StatusEmoji, &gender, &birthday, &privacyRaw, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if gender.Valid {
		u.Gender = int8(gender.Int64)
	}
	if birthday.Valid {
		s := birthday.Time.Format("2006-01-02")
		u.Birthday = &s
	}
	u.Privacy = model.ParsePrivacyJSON(privacyRaw)
	return u, nil
}

// EnsureBirthdayParse validates YYYY-MM-DD.
func ParseBirthday(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}
