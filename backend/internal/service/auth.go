package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"squirtlechat/internal/model"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/auth"
	"squirtlechat/pkg/idgen"
)

type AuthService struct {
	users     *store.UserStore
	idgen     *idgen.Generator
	jwtSecret string
}

func NewAuthService(users *store.UserStore, gen *idgen.Generator, secret string) *AuthService {
	return &AuthService{users: users, idgen: gen, jwtSecret: secret}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type LoginResult struct {
	User   *model.User `json:"user"`
	Tokens TokenPair   `json:"tokens"`
}

func (s *AuthService) Register(ctx context.Context, username, password, nickname string) (*LoginResult, error) {
	if username == "" || password == "" {
		return nil, errors.New("用户名和密码不能为空")
	}
	if nickname == "" {
		nickname = username
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		ID:           s.idgen.Next(),
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Privacy:      model.DefaultPrivacy(),
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	tokens, err := s.issueTokens(u.ID, u.Username, "")
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: tokens}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password, deviceID string) (*LoginResult, error) {
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("账号或密码错误，请重新输入")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("账号或密码错误，请重新输入")
	}
	if deviceID != "" {
		_ = s.users.UpsertDevice(ctx, u.ID, deviceID, deviceID)
	}
	u.PasswordHash = ""
	tokens, err := s.issueTokens(u.ID, u.Username, deviceID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: tokens}, nil
}

func (s *AuthService) issueTokens(userID int64, username, deviceID string) (TokenPair, error) {
	ttl := 24 * time.Hour
	access, err := auth.SignAccess(s.jwtSecret, userID, username, deviceID, ttl)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := auth.SignRefresh(s.jwtSecret, userID, 7*24*time.Hour)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(ttl.Seconds())}, nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID int64) (*model.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	u.PasswordHash = ""
	return u, nil
}

func (s *AuthService) GetPublicProfile(ctx context.Context, viewerID, targetID int64) (*model.PublicProfile, error) {
	u, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	pub := u.ApplyPrivacy(viewerID == targetID)
	return &pub, nil
}

type ProfileUpdateInput struct {
	Nickname *string
	Avatar   *string
	Gender   *int8
	Birthday *string
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, in ProfileUpdateInput) (*model.User, error) {
	patch := store.ProfilePatch{}
	if in.Nickname != nil {
		if *in.Nickname == "" {
			return nil, errors.New("昵称不能为空")
		}
		patch.Nickname = in.Nickname
	}
	if in.Avatar != nil {
		patch.Avatar = in.Avatar
	}
	if in.Gender != nil {
		if *in.Gender < 0 || *in.Gender > 2 {
			return nil, errors.New("性别取值无效")
		}
		patch.Gender = in.Gender
	}
	if in.Birthday != nil {
		if *in.Birthday == "" {
			empty := ""
			patch.Birthday = &empty
		} else {
			bd, err := store.ParseBirthday(*in.Birthday)
			if err != nil {
				return nil, errors.New("生日格式应为 YYYY-MM-DD")
			}
			patch.Birthday = &bd
		}
	}
	if patch.Nickname == nil && patch.Avatar == nil && patch.Gender == nil && patch.Birthday == nil {
		return s.GetProfile(ctx, userID)
	}
	if err := s.users.UpdateProfile(ctx, userID, patch); err != nil {
		return nil, errors.New("更新资料失败")
	}
	return s.GetProfile(ctx, userID)
}

func (s *AuthService) UpdatePrivacy(ctx context.Context, userID int64, p model.UserPrivacy) (*model.User, error) {
	if err := s.users.UpdatePrivacy(ctx, userID, p); err != nil {
		return nil, errors.New("更新隐私设置失败")
	}
	return s.GetProfile(ctx, userID)
}

func (s *AuthService) SearchUsers(ctx context.Context, q string, limit int) ([]*model.User, error) {
	if q == "" {
		return nil, errors.New("请输入搜索关键词")
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	list, err := s.users.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	for _, u := range list {
		u.PasswordHash = ""
	}
	return list, nil
}

func (s *AuthService) PublicProfilesForUsers(ctx context.Context, viewerID int64, userIDs []int64) ([]model.PublicProfile, error) {
	out := make([]model.PublicProfile, 0, len(userIDs))
	for _, id := range userIDs {
		u, err := s.users.GetByID(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, u.ApplyPrivacy(viewerID == id))
	}
	return out, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	userID, err := auth.ParseRefresh(s.jwtSecret, refreshToken)
	if err != nil {
		return nil, errors.New("登录已过期，请重新登录")
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	u.PasswordHash = ""
	tokens, err := s.issueTokens(u.ID, u.Username, "")
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Tokens: tokens}, nil
}

func (s *AuthService) ParseToken(token string) (*auth.Claims, error) {
	return auth.ParseAccess(s.jwtSecret, token)
}
