package handler

import (
	"io"
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/model"
	"squirtlechat/internal/service"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth  *service.AuthService
	files *service.FileService
	agent *service.AgentService
}

func NewAuthHandler(auth *service.AuthService, files *service.FileService, agent *service.AgentService) *AuthHandler {
	return &AuthHandler{auth: auth, files: files, agent: agent}
}

func (h *AuthHandler) Register(r *gin.RouterGroup) {
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
	r.POST("/auth/refresh", h.refresh)
	r.POST("/auth/logout", middleware.Auth(h.auth), h.logout)
	r.GET("/users/me", middleware.Auth(h.auth), h.me)
	r.PUT("/users/me", middleware.Auth(h.auth), h.updateMe)
	r.PUT("/users/me/privacy", middleware.Auth(h.auth), h.updatePrivacy)
	r.POST("/users/me/avatar", middleware.Auth(h.auth), h.uploadAvatar)
	r.GET("/users/search", middleware.Auth(h.auth), h.searchUsers)
	r.GET("/users/:id", middleware.Auth(h.auth), h.getUser)
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
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.auth.Login(c.Request.Context(), req.Username, req.Password, req.DeviceID)
	if err != nil {
		failUnauthorized(c, err)
		return
	}
	if h.agent != nil {
		_ = h.agent.EnsureForUser(c.Request.Context(), res.User.ID)
	}
	response.OK(c, res)
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
		Nickname *string `json:"nickname"`
		Avatar   *string `json:"avatar"`
		Gender   *int8   `json:"gender"`
		Birthday *string `json:"birthday"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	u, err := h.auth.UpdateProfile(c.Request.Context(), middleware.UserID(c), service.ProfileUpdateInput{
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Birthday: req.Birthday,
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
