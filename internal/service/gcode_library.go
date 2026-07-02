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

type GCodeLibraryListOptions struct {
	Query      string
	Material   string
	Profile    string
	Nozzle     string
	Layer      string
	TimeBucket string
	Usage      string
	Sort       string
	Page       int
	PageSize   int
}

type GCodeLibraryListResponse struct {
	Items    []model.GCodeLibraryFile `json:"items"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type GCodeLibraryUpdateOptions struct {
	DisplayName      string     `json:"display_name"`
	ParentSTLID      *uuid.UUID `json:"parent_stl_id"`
	MaterialType     string     `json:"material_type"`
	MaterialColor    string     `json:"material_color"`
	FilamentGrams    *float64   `json:"filament_grams"`
	EstimatedSeconds *int       `json:"estimated_seconds"`
	LayerHeight      *float64   `json:"layer_height"`
	NozzleDiameter   *float64   `json:"nozzle_diameter"`
	BedTemp          *float64   `json:"bed_temp"`
	NozzleTemp       *float64   `json:"nozzle_temp"`
	ThumbnailFileID  *uuid.UUID `json:"thumbnail_file_id"`
}

type GCodeLibraryService struct {
	repo    *repository.GCodeLibraryRepository
	stlRepo *repository.STLLibraryRepository
	files   *repository.FileRepository
	queue   *QueueService
	storage storage.Storage
}

func NewGCodeLibraryService(repos *repository.Repositories, store storage.Storage, queue *QueueService) *GCodeLibraryService {
	return &GCodeLibraryService{repo: repos.GCodeLibrary, stlRepo: repos.STLLibrary, files: repos.Files, queue: queue, storage: store}
}

func (s *GCodeLibraryService) List(ctx context.Context, opts GCodeLibraryListOptions) (*GCodeLibraryListResponse, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 50
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}
	items, total, err := s.repo.List(ctx, repository.GCodeLibraryListOptions{
		Query: opts.Query, Material: opts.Material, Profile: opts.Profile, Nozzle: opts.Nozzle, Layer: opts.Layer,
		TimeBucket: opts.TimeBucket, Usage: opts.Usage, Sort: opts.Sort, Page: opts.Page, PageSize: opts.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return &GCodeLibraryListResponse{Items: items, Total: total, Page: opts.Page, PageSize: opts.PageSize}, nil
}

func (s *GCodeLibraryService) Upload(ctx context.Context, filename string, reader io.Reader) (*model.GCodeLibraryFile, error) {
	return s.UploadWithParent(ctx, filename, reader, nil)
}

func (s *GCodeLibraryService) UploadWithParent(ctx context.Context, filename string, reader io.Reader, parentSTLID *uuid.UUID) (*model.GCodeLibraryFile, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".gcode" {
		return nil, fmt.Errorf("only .gcode files can be stored")
	}
	storagePath, hash, size, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, err
	}
	file := &model.File{Hash: hash, OriginalName: filename, ContentType: "text/x-gcode", SizeBytes: size, StoragePath: storagePath}
	existing, err := s.files.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.StoragePath != storagePath {
			if r, err := s.storage.Get(existing.StoragePath); err == nil {
				r.Close()
				_ = s.storage.Delete(storagePath)
			} else {
				existing.OriginalName = filename
				existing.ContentType = "text/x-gcode"
				existing.SizeBytes = size
				existing.StoragePath = storagePath
				if err := s.files.Update(ctx, existing); err != nil {
					return nil, err
				}
			}
		}
		file = existing
	} else if err := s.files.Create(ctx, file); err != nil {
		return nil, err
	}
	metadata, thumbnailID := s.queue.analyzeFile(ctx, file)
	entry := &model.GCodeLibraryFile{FileID: file.ID, DisplayName: strings.TrimSuffix(filename, filepath.Ext(filename)), Metadata: metadata, ThumbnailFileID: thumbnailID, ParentSTLID: parentSTLID}
	if parentSTLID != nil {
		children, err := s.repo.ListByParentSTL(ctx, *parentSTLID)
		if err != nil {
			return nil, err
		}
		entry.DefaultForSTL = len(children) == 0
	}
	if metadata != nil {
		entry.MaterialType = metadata.MaterialType
		entry.MaterialColor = metadata.MaterialColor
		entry.FilamentName = metadata.FilamentName
		entry.FilamentGrams = metadata.FilamentGrams
		entry.EstimatedSeconds = metadata.EstimatedSeconds
		entry.LayerHeight = metadata.LayerHeight
		entry.NozzleDiameter = metadata.NozzleDiameter
		entry.BedTemp = metadata.BedTemp
		entry.NozzleTemp = metadata.NozzleTemp
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *GCodeLibraryService) Update(ctx context.Context, id uuid.UUID, opts GCodeLibraryUpdateOptions) (*model.GCodeLibraryFile, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("gcode file not found")
	}
	if opts.DisplayName != "" {
		entry.DisplayName = opts.DisplayName
	}
	if opts.ThumbnailFileID != nil {
		entry.ThumbnailFileID = opts.ThumbnailFileID
		if entry.Metadata != nil {
			entry.Metadata.ThumbnailFileID = entry.ThumbnailFileID
		}
	}
	if opts.ParentSTLID != nil {
		entry.ParentSTLID = opts.ParentSTLID
	}
	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *GCodeLibraryService) SetParentSTL(ctx context.Context, id uuid.UUID, parentID *uuid.UUID) error {
	if parentID != nil {
		stl, err := s.repoSTL(ctx, *parentID)
		if err != nil {
			return err
		}
		if stl == nil {
			return fmt.Errorf("stl file not found")
		}
	}
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("gcode file not found")
	}
	oldParentID := entry.ParentSTLID
	wasDefault := entry.DefaultForSTL
	if err := s.repo.SetParentSTL(ctx, id, parentID); err != nil {
		return err
	}
	if oldParentID != nil && (parentID == nil || *oldParentID != *parentID) && wasDefault {
		items, err := s.repo.ListByParentSTL(ctx, *oldParentID)
		if err == nil && len(items) > 0 {
			return s.repo.SetDefaultForSTL(ctx, items[0].ID)
		}
	}
	return nil
}

func (s *GCodeLibraryService) SetDefaultForSTL(ctx context.Context, id uuid.UUID) error {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("gcode file not found")
	}
	if entry.ParentSTLID == nil {
		return fmt.Errorf("gcode is not linked to an STL")
	}
	return s.repo.SetDefaultForSTL(ctx, id)
}

func (s *GCodeLibraryService) repoSTL(ctx context.Context, id uuid.UUID) (*model.STLLibraryFile, error) {
	return s.stlRepo.GetByID(ctx, id)
}

func (s *GCodeLibraryService) Delete(ctx context.Context, id uuid.UUID) error {
	projects, err := s.repo.LinkedProjects(ctx, id)
	if err != nil {
		return err
	}
	if len(projects) > 0 {
		return fmt.Errorf("file is used by projects: %s", strings.Join(projects, ", "))
	}
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if entry != nil && entry.ParentSTLID != nil && entry.DefaultForSTL {
		items, err := s.repo.ListByParentSTL(ctx, *entry.ParentSTLID)
		if err == nil && len(items) > 0 {
			return s.repo.SetDefaultForSTL(ctx, items[0].ID)
		}
	}
	return nil
}

func (s *GCodeLibraryService) ListTags(ctx context.Context) ([]model.Tag, error) {
	return s.repo.ListTags(ctx)
}

func (s *GCodeLibraryService) CreateTag(ctx context.Context, name string, color string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("tag name is required")
	}
	if color == "" {
		color = "#64748b"
	}
	tag := &model.Tag{Name: name, Color: color}
	if err := s.repo.CreateTag(ctx, tag); err != nil {
		return nil, err
	}
	return tag, nil
}

func (s *GCodeLibraryService) DeleteTag(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTag(ctx, id)
}

func (s *GCodeLibraryService) AddTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	entry, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("gcode file not found")
	}
	if entry.ParentSTLID != nil {
		return fmt.Errorf("tags can only be added to root files")
	}
	return s.repo.AddTagToFile(ctx, fileID, tagID)
}

func (s *GCodeLibraryService) RemoveTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	return s.repo.RemoveTagFromFile(ctx, fileID, tagID)
}

type GCodeQueueOptions struct {
	AssignedSpoolID *uuid.UUID `json:"assigned_spool_id"`
	ProjectID       *uuid.UUID `json:"project_id"`
	MaterialType    string     `json:"material_type"`
	MaterialColor   string     `json:"material_color"`
	FilamentName    string     `json:"filament_name"`
	SourceType      string     `json:"source_type"`
}

func (s *GCodeLibraryService) AddToQueue(ctx context.Context, id uuid.UUID, options GCodeQueueOptions) (*model.QueueItem, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("gcode file not found")
	}
	file, err := s.files.GetByID(ctx, entry.FileID)
	if err != nil || file == nil {
		return nil, fmt.Errorf("file not found")
	}
	materialType := entry.MaterialType
	materialColor := entry.MaterialColor
	filamentName := entry.FilamentName
	if options.MaterialType != "" {
		materialType = strings.ToLower(strings.TrimSpace(options.MaterialType))
	}
	if options.MaterialColor != "" {
		materialColor = options.MaterialColor
	}
	if options.FilamentName != "" {
		filamentName = options.FilamentName
	}
	sourceType := model.QueueSourceLibrary
	if options.SourceType == string(model.QueueSourceProject) {
		sourceType = model.QueueSourceProject
	}
	item := &model.QueueItem{SourceType: sourceType, SourceID: &entry.ID, ProjectID: options.ProjectID, FileID: entry.FileID, FileName: file.OriginalName, DisplayName: entry.DisplayName, Status: model.QueueItemStatusQueued, AssignedPrinterID: s.queue.DefaultPrinterID(ctx), AssignedSpoolID: options.AssignedSpoolID, MaterialType: materialType, MaterialColor: materialColor, FilamentName: filamentName, FilamentGrams: entry.FilamentGrams, EstimatedSeconds: entry.EstimatedSeconds, LayerHeight: entry.LayerHeight, NozzleDiameter: entry.NozzleDiameter, BedTemp: entry.BedTemp, NozzleTemp: entry.NozzleTemp, ThumbnailFileID: entry.ThumbnailFileID, Metadata: entry.Metadata}
	s.queue.applyDefaultSpool(ctx, item)
	if item.DisplayName == "" {
		item.DisplayName = strings.TrimSuffix(file.OriginalName, filepath.Ext(file.OriginalName))
	}
	if err := s.queue.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
