package handler

import (
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/service"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type SyncHandler struct {
	svc  *service.SyncService
	auth *service.AuthService
}

func NewSyncHandler(svc *service.SyncService, auth *service.AuthService) *SyncHandler {
	return &SyncHandler{svc: svc, auth: auth}
}

func (h *SyncHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.GET("/sync", h.pull)
	g.POST("/sync/read", h.read)
	g.GET("/conversations/:id/read-state", h.readState)
}

func (h *SyncHandler) pull(c *gin.Context) {
	since, _ := strconv.ParseInt(c.Query("since_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	deviceID := c.Query("device_id")
	res, err := h.svc.Sync(c.Request.Context(), middleware.UserID(c), deviceID, since, limit)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, res)
}

func (h *SyncHandler) read(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversation_id" binding:"required"`
		ReadSeq        int64  `json:"read_seq" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), middleware.UserID(c), req.ConversationID, req.ReadSeq); err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *SyncHandler) readState(c *gin.Context) {
	convID := c.Param("id")
	list, err := h.svc.GetReadState(c.Request.Context(), middleware.UserID(c), convID)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"members": list})
}
