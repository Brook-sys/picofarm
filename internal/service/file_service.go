package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type FileService struct {
	repo    *repository.FileRepository
	storage storage.Storage
}

// GetByID retrieves a file by ID.
func (s *FileService) GetByID(ctx context.Context, id uuid.UUID) (*model.File, error) {
	return s.repo.GetByID(ctx, id)
}

// GetReader retrieves a file reader.
func (s *FileService) GetReader(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.File, error) {
	file, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if file == nil {
		return nil, nil, fmt.Errorf("file not found")
	}

	reader, err := s.storage.Get(file.StoragePath)
	if err != nil {
		return nil, nil, err
	}

	return reader, file, nil
}

// Upload saves a file (for custom thumbnails etc.) and returns the created File record.
func (s *FileService) Upload(ctx context.Context, filename string, reader io.Reader) (*model.File, error) {
	storagePath, hash, size, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, err
	}
	f := &model.File{Hash: hash, OriginalName: filename, ContentType: contentTypeFromFilename(filename), SizeBytes: size, StoragePath: storagePath}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func contentTypeFromFilename(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// getContentType returns MIME type for file extension.
func getContentType(ext string) string {
	switch ext {
	case "stl":
		return "model/stl"
	case "3mf":
		return "model/3mf"
	case "gcode":
		return "text/x-gcode"
	default:
		return "application/octet-stream"
	}
}

// ExpenseService handles expense and receipt processing.
