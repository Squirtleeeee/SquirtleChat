package store

import (
	"context"
	"database/sql"
)

type FileStore struct {
	db *sql.DB
}

func NewFileStore(db *sql.DB) *FileStore {
	return &FileStore{db: db}
}

func (s *FileStore) Create(ctx context.Context, id, uploaderID int64, filename string, size int64, contentType, objectKey string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO files (id, uploader_id, filename, size, content_type, object_key, status)
		VALUES (?, ?, ?, ?, ?, ?, 1)`, id, uploaderID, filename, size, contentType, objectKey)
	return err
}
