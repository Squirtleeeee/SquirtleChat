package handler

import (
	"squirtlechat/internal/middleware"
	"squirtlechat/internal/service"
	"squirtlechat/internal/service/linkpreview"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type LinkPreviewHandler struct {
	auth *service.AuthService
	lp   *linkpreview.Service
}

func NewLinkPreviewHandler(auth *service.AuthService, lp *linkpreview.Service) *LinkPreviewHandler {
	return &LinkPreviewHandler{auth: auth, lp: lp}
}

func (h *LinkPreviewHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.GET("/link-preview", h.get)
}

func (h *LinkPreviewHandler) get(c *gin.Context) {
	raw := c.Query("url")
	if raw == "" {
		response.Fail(c, 400, "请提供 url 参数")
		return
	}
	refresh := c.Query("refresh") == "1" || c.Query("refresh") == "true"
	p, err := h.lp.FetchOpt(c.Request.Context(), raw, refresh)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.OK(c, p)
}
