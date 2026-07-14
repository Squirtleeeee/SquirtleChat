package handler

import (
	"io"
	"os"
	"strings"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/service"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	files *service.FileService
	auth  *service.AuthService
}

func NewFileHandler(files *service.FileService, auth *service.AuthService) *FileHandler {
	return &FileHandler{files: files, auth: auth}
}

func (h *FileHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.POST("/files/upload", h.upload)
}

// RegisterPublic serves uploaded objects at GET /uploads/*filepath (no auth; matches Phase 5).
func (h *FileHandler) RegisterPublic(r *gin.Engine) {
	r.GET("/uploads/*filepath", h.serve)
}

func (h *FileHandler) serve(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("filepath"), "/")
	rc, size, ct, err := h.files.Open(c.Request.Context(), key)
	if err != nil {
		if os.IsNotExist(err) {
			c.Status(404)
			return
		}
		failInternal(c, err)
		return
	}
	defer rc.Close()
	if ct == "" {
		ct = "application/octet-stream"
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.DataFromReader(200, size, ct, rc, nil)
}

func (h *FileHandler) upload(c *gin.Context) {
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
	data, err := io.ReadAll(io.LimitReader(f, 20<<20))
	if err != nil {
		failInternal(c, err)
		return
	}
	res, err := h.files.Upload(c.Request.Context(), middleware.UserID(c), file.Filename, file.Header.Get("Content-Type"), data)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, res)
}
