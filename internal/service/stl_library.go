package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
)

type STLLibraryListOptions struct {
	Query    string
	Sort     string
	Page     int
	PageSize int
}

type STLLibraryListResponse struct {
	Items    []model.STLLibraryFile `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type STLLibraryUpdateOptions struct {
	DisplayName string `json:"display_name"`
}

type STLLibraryService struct {
	repo      *repository.STLLibraryRepository
	gcodeRepo *repository.GCodeLibraryRepository
	files     *repository.FileRepository
	storage   storage.Storage
}

func NewSTLLibraryService(repos *repository.Repositories, store storage.Storage) *STLLibraryService {
	return &STLLibraryService{repo: repos.STLLibrary, gcodeRepo: repos.GCodeLibrary, files: repos.Files, storage: store}
}

func (s *STLLibraryService) List(ctx context.Context, opts STLLibraryListOptions) (*STLLibraryListResponse, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 50
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}
	items, total, err := s.repo.List(ctx, repository.STLLibraryListOptions{Query: opts.Query, Sort: opts.Sort, Page: opts.Page, PageSize: opts.PageSize})
	if err != nil {
		return nil, err
	}
	return &STLLibraryListResponse{Items: items, Total: total, Page: opts.Page, PageSize: opts.PageSize}, nil
}

func (s *STLLibraryService) Upload(ctx context.Context, filename string, reader io.Reader, thumbnail io.Reader) (*model.STLLibraryFile, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".stl" {
		return nil, fmt.Errorf("only .stl files can be stored")
	}
	storagePath, hash, size, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, err
	}
	file := &model.File{Hash: hash, OriginalName: filename, ContentType: "model/stl", SizeBytes: size, StoragePath: storagePath}
	existing, err := s.files.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.StoragePath != storagePath {
			s.storage.Delete(storagePath)
		}
		file = existing
	} else if err := s.files.Create(ctx, file); err != nil {
		return nil, err
	}
	thumbnailID, err := s.saveThumbnail(ctx, filename, thumbnail)
	if err != nil {
		return nil, err
	}
	entry := &model.STLLibraryFile{FileID: file.ID, DisplayName: strings.TrimSuffix(filename, filepath.Ext(filename)), SizeBytes: size, ThumbnailFileID: thumbnailID}
	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}
	entry.FileName = file.OriginalName
	return entry, nil
}

func (s *STLLibraryService) saveThumbnail(ctx context.Context, filename string, thumbnail io.Reader) (*uuid.UUID, error) {
	if thumbnail == nil {
		return nil, nil
	}
	thumbName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".png"
	storagePath, hash, size, err := s.storage.Save(thumbName, thumbnail)
	if err != nil {
		return nil, err
	}
	file := &model.File{Hash: hash, OriginalName: thumbName, ContentType: "image/png", SizeBytes: size, StoragePath: storagePath}
	existing, err := s.files.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.StoragePath != storagePath {
			s.storage.Delete(storagePath)
		}
		return &existing.ID, nil
	}
	if err := s.files.Create(ctx, file); err != nil {
		return nil, err
	}
	return &file.ID, nil
}

func (s *STLLibraryService) UpdateThumbnail(ctx context.Context, id uuid.UUID, thumbnail io.Reader) (*model.STLLibraryFile, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("stl file not found")
	}
	thumbnailID, err := s.saveThumbnail(ctx, entry.FileName, thumbnail)
	if err != nil {
		return nil, err
	}
	entry.ThumbnailFileID = thumbnailID
	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *STLLibraryService) Update(ctx context.Context, id uuid.UUID, opts STLLibraryUpdateOptions) (*model.STLLibraryFile, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("stl file not found")
	}
	if strings.TrimSpace(opts.DisplayName) != "" {
		entry.DisplayName = strings.TrimSpace(opts.DisplayName)
	}
	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *STLLibraryService) Library(ctx context.Context) (*model.FileLibraryResponse, error) {
	stls, _, err := s.repo.List(ctx, repository.STLLibraryListOptions{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	root, err := s.gcodeRepo.ListRoot(ctx)
	if err != nil {
		return nil, err
	}
	for i := range stls {
		children, err := s.gcodeRepo.ListByParentSTL(ctx, stls[i].ID)
		if err != nil {
			return nil, err
		}
		stls[i].GCodes = children
	}
	return &model.FileLibraryResponse{STLFiles: stls, RootGCodeFiles: root}, nil
}

func (s *STLLibraryService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.gcodeRepo.ClearParentSTL(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *STLLibraryService) AddTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	return s.repo.AddTagToFile(ctx, fileID, tagID)
}

func (s *STLLibraryService) RemoveTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	return s.repo.RemoveTagFromFile(ctx, fileID, tagID)
}
