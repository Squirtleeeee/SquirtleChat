package model

import (
	"encoding/json"
	"time"
)

type UserPrivacy struct {
	ShowNickname bool `json:"show_nickname"`
	ShowGender   bool `json:"show_gender"`
	ShowBirthday bool `json:"show_birthday"`
	ShowAvatar   bool `json:"show_avatar"`
}

func DefaultPrivacy() UserPrivacy {
	return UserPrivacy{ShowNickname: true, ShowGender: false, ShowBirthday: false, ShowAvatar: true}
}

type User struct {
	ID           int64       `json:"id,string"`
	Username     string      `json:"username"`
	PasswordHash string      `json:"-"`
	Nickname     string      `json:"nickname"`
	Avatar       string      `json:"avatar"`
	StatusText   string      `json:"status_text"`
	StatusEmoji  string      `json:"status_emoji"`
	Gender       int8        `json:"gender"`
	Birthday     *string     `json:"birthday,omitempty"`
	Privacy      UserPrivacy `json:"privacy"`
	CreatedAt    time.Time   `json:"created_at"`
}

// PublicProfile is visible to other users (privacy-filtered).
type PublicProfile struct {
	ID          int64  `json:"id,string"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	StatusText  string `json:"status_text,omitempty"`
	StatusEmoji string `json:"status_emoji,omitempty"`
	Gender      int8   `json:"gender,omitempty"`
	Birthday    string `json:"birthday,omitempty"`
}

func (u *User) ApplyPrivacy(viewerIsSelf bool) PublicProfile {
	if viewerIsSelf {
		bday := ""
		if u.Birthday != nil {
			bday = *u.Birthday
		}
		return PublicProfile{
			ID: u.ID, Username: u.Username, Nickname: u.Nickname,
			Avatar: u.Avatar, StatusText: u.StatusText, StatusEmoji: u.StatusEmoji,
			Gender: u.Gender, Birthday: bday,
		}
	}
	out := PublicProfile{
		ID: u.ID, Username: u.Username,
		StatusText: u.StatusText, StatusEmoji: u.StatusEmoji,
	}
	if u.Privacy.ShowNickname && u.Nickname != "" {
		out.Nickname = u.Nickname
	}
	if u.Privacy.ShowAvatar && u.Avatar != "" {
		out.Avatar = u.Avatar
	}
	if u.Privacy.ShowGender && u.Gender > 0 {
		out.Gender = u.Gender
	}
	if u.Privacy.ShowBirthday && u.Birthday != nil && *u.Birthday != "" {
		out.Birthday = *u.Birthday
	}
	return out
}

func ParsePrivacyJSON(raw []byte) UserPrivacy {
	p := DefaultPrivacy()
	if len(raw) == 0 {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	return p
}

type Message struct {
	ID             int64      `json:"msg_id"`
	ConversationID string     `json:"conversation_id"`
	FromUserID     int64      `json:"from_user_id,string"`
	Seq            int64      `json:"seq"`
	MsgType        int8       `json:"msg_type"`
	Content        string     `json:"content"`
	ClientMsgID    string     `json:"client_msg_id"`
	CreatedAt      time.Time  `json:"created_at"`
	EditedAt       *time.Time `json:"edited_at,omitempty"`
}

const (
	ConvTypeDirect = 1
	ConvTypeGroup  = 2
	MsgTypeText    = 1
	MsgTypeImage   = 2
	MsgTypeFile    = 3
	MsgTypeSystem  = 4
	MsgTypeAudio   = 5
	MsgTypePoll    = 6
)
