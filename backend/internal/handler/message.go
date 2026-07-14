package handler

import (
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/model"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	msg  *service.MessageService
	auth *service.AuthService
}

func NewMessageHandler(msg *service.MessageService, auth *service.AuthService) *MessageHandler {
	return &MessageHandler{msg: msg, auth: auth}
}

func (h *MessageHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.GET("/conversations", h.listConversations)
	g.GET("/conversations/:id/messages/search", h.search)
	g.GET("/conversations/:id/messages", h.list)
	g.POST("/conversations/:id/messages/:msg_id/recall", h.recall)
}

func (h *MessageHandler) listConversations(c *gin.Context) {
	list, err := h.msg.ListConversations(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"conversations": list})
}

func (h *MessageHandler) list(c *gin.Context) {
	convID := c.Param("id")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	around, _ := strconv.ParseInt(c.Query("around_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.ListMessages(c.Request.Context(), middleware.UserID(c), convID, before, around, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	response.OK(c, gin.H{"messages": msgs})
}

func (h *MessageHandler) search(c *gin.Context) {
	convID := c.Param("id")
	q := c.Query("q")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.SearchMessages(c.Request.Context(), middleware.UserID(c), convID, q, before, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "请输入搜索关键词" || err.Error() == "搜索关键词过长" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	response.OK(c, gin.H{"messages": msgs, "q": q})
}

func (h *MessageHandler) recall(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	msg, err := h.msg.RecallMessage(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权操作该会话" || err.Error() == "只能撤回自己的消息" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err == store.ErrNotFound || err.Error() == "消息已撤回" {
			response.Fail(c, 404, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"message": msg})
}
