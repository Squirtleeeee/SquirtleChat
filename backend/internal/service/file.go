package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"squirtlechat/internal/store"
	"squirtlechat/pkg/config"
	"squirtlechat/pkg/idgen"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileService struct {
	files  *store.FileStore
	gen    *idgen.Generator
	cfg    *config.Config
	local  string
	minio  *minio.Client
	bucket string
}

func NewFileService(files *store.FileStore, gen *idgen.Generator, cfg *config.Config) *FileService {
	s := &FileService{files: files, gen: gen, cfg: cfg, local: "data/uploads", bucket: cfg.MinioBucket}
	_ = os.MkdirAll(s.local, 0o755)

	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: false,
	})
	if err == nil {
		ctx := context.Background()
		if _, err := client.BucketExists(ctx, cfg.MinioBucket); err == nil {
			s.minio = client
			exists, _ := client.BucketExists(ctx, cfg.MinioBucket)
			if !exists {
				_ = client.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
			}
		}
	}
	return s
}

func (s *FileService) LocalDir() string {
	if s.minio != nil {
		return ""
	}
	return s.local
}

type UploadResult struct {
	FileID  int64  `json:"file_id"`
	URL     string `json:"url"`
	Name    string `json:"filename"`
	Size    int64  `json:"size"`
	Content string `json:"content_type"`
}

func (s *FileService) Upload(ctx context.Context, userID int64, name, contentType string, data []byte) (*UploadResult, error) {
	id := s.gen.Next()
	key := fmt.Sprintf("%d_%s", id, filepath.Base(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var url string
	if s.minio != nil {
		_, err := s.minio.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			return nil, err
		}
	} else {
		path := filepath.Join(s.local, key)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, err
		}
	}
	// Always expose via gateway /uploads so browsers can fetch without MinIO public bucket policy.
	url = "/uploads/" + key

	if err := s.files.Create(ctx, id, userID, name, int64(len(data)), contentType, key); err != nil {
		return nil, err
	}
	return &UploadResult{FileID: id, URL: url, Name: name, Size: int64(len(data)), Content: contentType}, nil
}

// Open returns a readable object for GET /uploads/{key} (MinIO or local fallback).
func (s *FileService) Open(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	if key == "" || strings.Contains(key, "..") {
		return nil, 0, "", os.ErrNotExist
	}
	if s.minio != nil {
		obj, err := s.minio.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return nil, 0, "", err
		}
		stat, err := obj.Stat()
		if err != nil {
			_ = obj.Close()
			return nil, 0, "", err
		}
		ct := stat.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		return obj, stat.Size, ct, nil
	}
	path := filepath.Join(s.local, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, "", err
	}
	ct := "application/octet-stream"
	if ext := filepath.Ext(key); ext != "" {
		ct = mimeTypeByExt(ext)
	}
	return f, info.Size(), ct, nil
}

func mimeTypeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
