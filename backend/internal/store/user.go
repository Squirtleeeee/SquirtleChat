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

const userSelectCols = `id, username, password_hash, nickname, avatar, gender, birthday, privacy_json, created_at`

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
	Nickname *string
	Avatar   *string
	Gender   *int8
	Birthday *string
}

func (s *UserStore) UpdateProfile(ctx context.Context, userID int64, patch ProfilePatch) error {
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	nickname := u.Nickname
	avatar := u.Avatar
	gender := u.Gender
	var birthday *string = u.Birthday
	if patch.Nickname != nil {
		nickname = *patch.Nickname
	}
	if patch.Avatar != nil {
		avatar = *patch.Avatar
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
		UPDATE users SET nickname = ?, avatar = ?, gender = ?, birthday = ?, updated_at = NOW(3)
		WHERE id = ?`, nickname, avatar, gender, bdayVal, userID)
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
		ON DUPLICATE KEY UPDATE device_name = VALUES(device_name), last_active_at = NOW(3)`,
		userID, deviceID, deviceName)
	return err
}

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	var gender sql.NullInt64
	var birthday sql.NullTime
	var privacyRaw []byte
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.Avatar, &gender, &birthday, &privacyRaw, &u.CreatedAt)
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
	err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.Avatar, &gender, &birthday, &privacyRaw, &u.CreatedAt)
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
