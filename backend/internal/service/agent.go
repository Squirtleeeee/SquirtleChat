package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"squirtlechat/internal/agent"
	"squirtlechat/internal/model"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/config"
	"squirtlechat/pkg/idgen"
)

const (
	AgentUsername  = "squirtle_ai"
	AgentNickname  = "杰尼龟龟"
	AgentAvatarURL = "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/other/official-artwork/7.png"
)

type AgentService struct {
	users   *store.UserStore
	friend  *store.FriendStore
	msgs    *store.MessageStore
	msgSvc  *MessageService
	gen     *idgen.Generator
	llm     *agent.Client
	botID   int64
	botOnce sync.Once
	botErr  error
}

func NewAgentService(
	users *store.UserStore,
	friend *store.FriendStore,
	msgs *store.MessageStore,
	msgSvc *MessageService,
	gen *idgen.Generator,
	cfg *config.Config,
) *AgentService {
	return &AgentService{
		users:  users,
		friend: friend,
		msgs:   msgs,
		msgSvc: msgSvc,
		gen:    gen,
		llm:    agent.NewClient(cfg.LLMAPIBase, cfg.LLMAPIKey, cfg.LLMModel),
	}
}

func (s *AgentService) Init(ctx context.Context) error {
	botID, err := s.botUserID(ctx)
	if err != nil || botID == 0 {
		return err
	}
	return s.syncProfile(ctx, botID)
}

func (s *AgentService) syncProfile(ctx context.Context, botID int64) error {
	nick := AgentNickname
	avatar := AgentAvatarURL
	return s.users.UpdateProfile(ctx, botID, store.ProfilePatch{Nickname: &nick, Avatar: &avatar})
}

func (s *AgentService) BotUserID(ctx context.Context) (int64, error) {
	return s.botUserID(ctx)
}

func (s *AgentService) botUserID(ctx context.Context) (int64, error) {
	s.botOnce.Do(func() {
		u, err := s.users.GetByUsername(ctx, AgentUsername)
		if err == nil {
			s.botID = u.ID
			return
		}
		if !errors.Is(err, store.ErrNotFound) {
			s.botErr = err
			return
		}
		var pw [24]byte
		_, _ = rand.Read(pw[:])
		hash, herr := bcrypt.GenerateFromPassword(pw[:], bcrypt.DefaultCost)
		if herr != nil {
			s.botErr = herr
			return
		}
		id := s.gen.Next()
		nu := &model.User{
			ID:           id,
			Username:     AgentUsername,
			PasswordHash: string(hash),
			Nickname:     AgentNickname,
			Privacy:      model.DefaultPrivacy(),
		}
		if err := s.users.Create(ctx, nu); err != nil {
			// race: another instance created the bot
			if u2, err2 := s.users.GetByUsername(ctx, AgentUsername); err2 == nil {
				s.botID = u2.ID
				return
			}
			s.botErr = err
			return
		}
		s.botID = id
	})
	return s.botID, s.botErr
}

func (s *AgentService) EnsureForUser(ctx context.Context, userID int64) error {
	botID, err := s.botUserID(ctx)
	if err != nil || botID == 0 || userID == botID {
		return err
	}
	return s.friend.EnsureFriendship(ctx, userID, botID)
}

func (s *AgentService) IsAgentUserID(id int64) bool {
	return id > 0 && id == s.botID
}

func (s *AgentService) OnUserMessage(ctx context.Context, evt *store.SentEvent) {
	if evt == nil || evt.MsgType != model.MsgTypeText {
		return
	}
	botID, err := s.botUserID(ctx)
	if err != nil || botID == 0 {
		return
	}
	if evt.FromUserID == botID {
		return
	}
	peerID := int64(0)
	for _, uid := range evt.ToUserIDs {
		if uid == botID {
			peerID = evt.FromUserID
			break
		}
	}
	if peerID == 0 {
		return
	}
	go s.replyAsync(peerID, botID, evt.ConversationID, strings.TrimSpace(evt.Content))
}

func (s *AgentService) replyAsync(peerID, botID int64, convID, userText string) {
	ctx := context.Background()
	if userText == "" {
		return
	}

	if s.msgSvc.onTyping != nil {
		s.msgSvc.onTyping(ctx, &TypingEvent{
			ConversationID: convID,
			FromUserID:     botID,
			Typing:         true,
			ToUserIDs:      []int64{peerID},
		})
		defer s.msgSvc.onTyping(ctx, &TypingEvent{
			ConversationID: convID,
			FromUserID:     botID,
			Typing:         false,
			ToUserIDs:      []int64{peerID},
		})
	}

	reply, err := s.generateReply(ctx, botID, peerID, convID, userText)
	if err != nil {
		log.Printf("agent reply (model=%s base=%s): %v", s.llm.Model, s.llm.BaseURL, err)
		if !s.llm.Enabled() {
			reply = "呜…龟龟还想跟你好好唠嗑呢，但服务端还没配置 LLM_API_KEY。让管理员在 deploy/llm.env 填好密钥后重启 gateway，我就回来啦～"
		} else {
			reply = "呜呜，龟龟这会儿有点卡壳了…稍后再来找我聊聊好不好？"
		}
	}
	if err := s.msgSvc.InjectAgentReply(ctx, botID, peerID, reply); err != nil {
		log.Printf("agent inject: %v", err)
	}
}

func (s *AgentService) generateReply(ctx context.Context, botID, peerID int64, convID, userText string) (string, error) {
	msgs, err := s.msgs.ListByConversation(ctx, convID, 0, 12)
	if err != nil {
		return "", err
	}
	// chronological
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	history := make([]agent.ChatMessage, 0, len(msgs)+2)
	history = append(history, agent.ChatMessage{
		Role:    "system",
		Content: agent.SystemPrompt,
	})
	for _, m := range msgs {
		if m.MsgType != model.MsgTypeText {
			continue
		}
		text := strings.TrimSpace(m.Content)
		if text == "" || text == "[已撤回]" || isAgentFallbackText(text) {
			continue
		}
		role := "user"
		if m.FromUserID == botID {
			role = "assistant"
		}
		history = append(history, agent.ChatMessage{Role: role, Content: text})
	}
	// ensure latest user turn present (race with DB read)
	if len(history) == 0 || history[len(history)-1].Content != userText {
		history = append(history, agent.ChatMessage{Role: "user", Content: userText})
	}

	if !s.llm.Enabled() {
		return "", errors.New("llm disabled")
	}
	return s.llm.Chat(ctx, history)
}

func (s *AgentService) WelcomeHint() string {
	if s.llm.Enabled() {
		return "龟龟在线上啦～想吐槽、想唠嗑、想问啥都行，慢慢来，我听着呢 🐢"
	}
	return fmt.Sprintf("我是%s，配置好 LLM_API_KEY 之后，就能陪你慢慢聊啦。", AgentNickname)
}

func (s *AgentService) LLMEnabled() bool {
	return s.llm.Enabled()
}

func (s *AgentService) LLMBase() string {
	if s.llm == nil {
		return ""
	}
	return s.llm.BaseURL
}

func (s *AgentService) LLMModel() string {
	if s.llm == nil {
		return ""
	}
	return s.llm.Model
}

func isAgentFallbackText(text string) bool {
	return strings.Contains(text, "LLM_API_KEY") ||
		strings.Contains(text, "暂时无法回复") ||
		strings.Contains(text, "有点卡壳") ||
		strings.Contains(text, "deploy/llm.env")
}
