package handler

import (
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/service"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	agent *service.AgentService
	auth  *service.AuthService
}

func NewAgentHandler(agent *service.AgentService, auth *service.AuthService) *AgentHandler {
	return &AgentHandler{agent: agent, auth: auth}
}

func (h *AgentHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.GET("/agent/info", h.info)
	g.POST("/agent/ensure", h.ensure)
}

func (h *AgentHandler) info(c *gin.Context) {
	userID := middleware.UserID(c)
	if err := h.agent.EnsureForUser(c.Request.Context(), userID); err != nil {
		failInternal(c, err)
		return
	}
	botID, err := h.agent.BotUserID(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}
	pub, err := h.auth.GetPublicProfile(c.Request.Context(), userID, botID)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{
		"user":      pub,
		"llm":       h.agent.LLMEnabled(),
		"llm_base":  h.agent.LLMBase(),
		"llm_model": h.agent.LLMModel(),
		"user_id":   strconv.FormatInt(botID, 10),
	})
}

func (h *AgentHandler) ensure(c *gin.Context) {
	userID := middleware.UserID(c)
	if err := h.agent.EnsureForUser(c.Request.Context(), userID); err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}
