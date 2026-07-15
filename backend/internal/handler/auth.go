package handler

import (
	"io"
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/model"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/response"
	"squirtlechat/pkg/routing"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth       *service.AuthService
	files      *service.FileService
	agent      *service.AgentService
	kickDevice func(userID int64, deviceID string)
	router     *routing.Router
}

func NewAuthHandler(
	auth *service.AuthService,
	files *service.FileService,
	agent *service.AgentService,
	kickDevice func(userID int64, deviceID string),
	router *routing.Router,
) *AuthHandler {
	return &AuthHandler{auth: auth, files: files, agent: agent, kickDevice: kickDevice, router: router}
}

func (h *AuthHandler) Register(r *gin.RouterGroup) {
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
	r.POST("/auth/refresh", h.refresh)
	r.POST("/auth/logout", middleware.Auth(h.auth), h.logout)
	r.GET("/users/me", middleware.Auth(h.auth), h.me)
	r.PUT("/users/me", middleware.Auth(h.auth), h.updateMe)
	r.PUT("/users/me/password", middleware.Auth(h.auth), h.changePassword)
	r.PUT("/users/me/privacy", middleware.Auth(h.auth), h.updatePrivacy)
	r.POST("/users/me/avatar", middleware.Auth(h.auth), h.uploadAvatar)
	r.GET("/users/me/devices", middleware.Auth(h.auth), h.listDevices)
	r.DELETE("/users/me/devices/:deviceId", middleware.Auth(h.auth), h.revokeDevice)
	r.GET("/users/me/chat-prefs", middleware.Auth(h.auth), h.getChatPrefs)
	r.PUT("/users/me/chat-prefs", middleware.Auth(h.auth), h.putChatPrefs)
	r.GET("/users/me/drafts", middleware.Auth(h.auth), h.listDrafts)
	r.PUT("/users/me/drafts", middleware.Auth(h.auth), h.putDraft)
	r.POST("/users/presence", middleware.Auth(h.auth), h.presence)
	r.GET("/users/search", middleware.Auth(h.auth), h.searchUsers)
	r.GET("/users/:id", middleware.Auth(h.auth), h.getUser)
}

func (h *AuthHandler) presence(c *gin.Context) {
	if h.router == nil {
		response.OK(c, gin.H{"online": map[string]bool{}})
		return
	}
	var req struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if len(req.UserIDs) > 100 {
		response.Fail(c, 400, "一次最多查询 100 个用户")
		return
	}
	ids := make([]int64, 0, len(req.UserIDs))
	for _, s := range req.UserIDs {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	flags, err := h.router.BatchOnline(c.Request.Context(), ids)
	if err != nil {
		failInternal(c, err)
		return
	}
	out := make(map[string]bool, len(flags))
	for id, on := range flags {
		out[strconv.FormatInt(id, 10)] = on
	}
	response.OK(c, gin.H{"online": out})
}

func (h *AuthHandler) register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.auth.Register(c.Request.Context(), req.Username, req.Password, req.Nickname)
	if err != nil {
		failConflict(c, err)
		return
	}
	if h.agent != nil {
		_ = h.agent.EnsureForUser(c.Request.Context(), res.User.ID)
	}
	response.OK(c, res)
}

func (h *AuthHandler) login(c *gin.Context) {
	var req struct {
		Username   string `json:"username" binding:"required"`
		Password   string `json:"password" binding:"required"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if req.DeviceName == "" {
		req.DeviceName = c.GetHeader("User-Agent")
		if len(req.DeviceName) > 120 {
			req.DeviceName = req.DeviceName[:120]
		}
	}
	res, err := h.auth.Login(c.Request.Context(), req.Username, req.Password, req.DeviceID, req.DeviceName)
	if err != nil {
		failUnauthorized(c, err)
		return
	}
	if h.agent != nil {
		_ = h.agent.EnsureForUser(c.Request.Context(), res.User.ID)
	}
	response.OK(c, res)
}

func (h *AuthHandler) listDevices(c *gin.Context) {
	current := c.GetHeader("X-Device-Id")
	if current == "" {
		current = c.Query("current_device_id")
	}
	list, err := h.auth.ListDevices(c.Request.Context(), middleware.UserID(c), current)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"devices": list})
}

func (h *AuthHandler) revokeDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")
	userID := middleware.UserID(c)
	if err := h.auth.RevokeDevice(c.Request.Context(), userID, deviceID); err != nil {
		failConflict(c, err)
		return
	}
	if h.kickDevice != nil {
		h.kickDevice(userID, deviceID)
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *AuthHandler) getChatPrefs(c *gin.Context) {
	prefs, err := h.auth.GetChatPrefs(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, prefs)
}

func (h *AuthHandler) putChatPrefs(c *gin.Context) {
	var req store.ChatPrefs
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.auth.SaveChatPrefs(c.Request.Context(), middleware.UserID(c), req); err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *AuthHandler) listDrafts(c *gin.Context) {
	items, err := h.auth.ListDraftItems(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	drafts := map[string]string{}
	for _, it := range items {
		drafts[it.ConversationID] = it.Content
	}
	response.OK(c, gin.H{"drafts": drafts, "items": items})
}

func (h *AuthHandler) putDraft(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversation_id" binding:"required"`
		Content        string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.auth.SaveDraft(c.Request.Context(), middleware.UserID(c), req.ConversationID, req.Content); err != nil {
		if err.Error() == "会话 ID 无效" || err.Error() == "草稿过长" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *AuthHandler) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.auth.ChangePassword(c.Request.Context(), middleware.UserID(c), req.OldPassword, req.NewPassword); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *AuthHandler) me(c *gin.Context) {
	u, err := h.auth.GetProfile(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failUnauthorized(c, err)
		return
	}
	response.OK(c, gin.H{"user": u})
}

func (h *AuthHandler) updateMe(c *gin.Context) {
	var req struct {
		Nickname    *string `json:"nickname"`
		Avatar      *string `json:"avatar"`
		StatusText  *string `json:"status_text"`
		StatusEmoji *string `json:"status_emoji"`
		Gender      *int8   `json:"gender"`
		Birthday    *string `json:"birthday"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	u, err := h.auth.UpdateProfile(c.Request.Context(), middleware.UserID(c), service.ProfileUpdateInput{
		Nickname:    req.Nickname,
		Avatar:      req.Avatar,
		StatusText:  req.StatusText,
		StatusEmoji: req.StatusEmoji,
		Gender:      req.Gender,
		Birthday:    req.Birthday,
	})
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"user": u})
}

func (h *AuthHandler) updatePrivacy(c *gin.Context) {
	var req model.UserPrivacy
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	u, err := h.auth.UpdatePrivacy(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"user": u})
}

func (h *AuthHandler) uploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failParam(c, err)
		return
	}
	f, err := file.Open()
	if err != nil {
		failInternal(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 5<<20))
	if err != nil {
		failInternal(c, err)
		return
	}
	ct := file.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	res, err := h.files.Upload(c.Request.Context(), middleware.UserID(c), file.Filename, ct, data)
	if err != nil {
		failInternal(c, err)
		return
	}
	u, err := h.auth.UpdateProfile(c.Request.Context(), middleware.UserID(c), service.ProfileUpdateInput{
		Avatar: &res.URL,
	})
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"user": u, "url": res.URL})
}

func (h *AuthHandler) getUser(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		failParam(c, err)
		return
	}
	viewer := middleware.UserID(c)
	pub, err := h.auth.GetPublicProfile(c.Request.Context(), viewer, targetID)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"user": pub})
}

func (h *AuthHandler) searchUsers(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.auth.SearchUsers(c.Request.Context(), q, limit)
	if err != nil {
		failParam(c, err)
		return
	}
	viewer := middleware.UserID(c)
	out := make([]model.PublicProfile, 0, len(list))
	for _, u := range list {
		if u.ID == viewer {
			continue
		}
		out = append(out, u.ApplyPrivacy(false))
	}
	response.OK(c, gin.H{"users": out})
}

func (h *AuthHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		failUnauthorized(c, err)
		return
	}
	response.OK(c, res)
}

func (h *AuthHandler) logout(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}
