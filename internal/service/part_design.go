package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/Brook-sys/picofarm/internal/threemf"
	"github.com/google/uuid"
)

type PartService struct {
	repo        *repository.PartRepository
	designRepo  *repository.DesignRepository
	projectRepo *repository.ProjectRepository
	tagRepo     *repository.TagRepository
	stlRepo     *repository.STLLibraryRepository
}

// Create creates a new part.
func (s *PartService) Create(ctx context.Context, p *model.Part) error {
	if p.Name == "" {
		return fmt.Errorf("part name is required")
	}
	if p.ProjectID == uuid.Nil {
		return fmt.Errorf("project ID is required")
	}
	return s.repo.Create(ctx, p)
}

// GetByID retrieves a part by ID.
func (s *PartService) GetByID(ctx context.Context, id uuid.UUID) (*model.Part, error) {
	return s.repo.GetByID(ctx, id)
}

// ListByProject retrieves all parts for a project.
func (s *PartService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.Part, error) {
	return s.repo.ListByProject(ctx, projectID)
}

// Update updates a part.
func (s *PartService) Update(ctx context.Context, p *model.Part) error {
	return s.repo.Update(ctx, p)
}

// Delete removes a part.
func (s *PartService) Delete(ctx context.Context, id uuid.UUID) error {
	part, _ := s.repo.GetByID(ctx, id)
	var designs []model.Design
	if s.designRepo != nil {
		designs, _ = s.designRepo.ListByPart(ctx, id)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if part == nil || s.projectRepo == nil || s.tagRepo == nil || s.stlRepo == nil || s.designRepo == nil {
		return nil
	}
	project, err := s.projectRepo.GetByID(ctx, part.ProjectID)
	if err != nil || project == nil {
		return err
	}
	tag, err := s.tagRepo.GetByName(ctx, projectTagName(project.Name))
	if err != nil || tag == nil {
		return err
	}
	for _, design := range designs {
		if design.FileType != model.FileTypeSTL {
			continue
		}
		stillUsed, err := s.designRepo.ProjectHasFileDesign(ctx, project.ID, design.FileID)
		if err != nil || stillUsed {
			if err != nil {
				return err
			}
			continue
		}
		stl, err := s.stlRepo.GetByFileID(ctx, design.FileID)
		if err != nil {
			return err
		}
		if stl != nil {
			if err := s.stlRepo.RemoveTagFromFile(ctx, stl.ID, tag.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// DesignService handles design business logic.
type DesignService struct {
	repo        *repository.DesignRepository
	partRepo    *repository.PartRepository
	projectRepo *repository.ProjectRepository
	tagRepo     *repository.TagRepository
	fileRepo    *repository.FileRepository
	gcodeRepo   *repository.GCodeLibraryRepository
	stlRepo     *repository.STLLibraryRepository
	storage     storage.Storage
}

// Create creates a new design version with file upload.
func (s *DesignService) Create(ctx context.Context, partID uuid.UUID, filename string, reader io.Reader, notes string) (*model.Design, error) {
	if partID == uuid.Nil {
		return nil, fmt.Errorf("part ID is required")
	}

	// Determine file type from extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	var fileType model.FileType
	switch ext {
	case "stl":
		fileType = model.FileTypeSTL
	case "3mf":
		fileType = model.FileType3MF
	case "gcode":
		fileType = model.FileTypeGCODE
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	// Save file to storage
	storagePath, hash, size, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Check for existing file with same hash (deduplication)
	existingFile, err := s.fileRepo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	var fileID uuid.UUID
	if existingFile != nil {
		fileID = existingFile.ID
		// Remove duplicate file from storage
		s.storage.Delete(storagePath) //nolint:errcheck // best-effort duplicate cleanup
		storagePath = existingFile.StoragePath
	} else {
		// Create new file record
		file := &model.File{
			Hash:         hash,
			OriginalName: filename,
			ContentType:  getContentType(ext),
			SizeBytes:    size,
			StoragePath:  storagePath,
		}
		if err := s.fileRepo.Create(ctx, file); err != nil {
			return nil, fmt.Errorf("failed to create file record: %w", err)
		}
		fileID = file.ID
	}

	// Create design record
	design := &model.Design{
		PartID:        partID,
		FileID:        fileID,
		FileName:      filename,
		FileHash:      hash,
		FileSizeBytes: size,
		FileType:      fileType,
		Notes:         notes,
	}

	// Extract slicer metadata from 3MF files
	if fileType == model.FileType3MF {
		fullPath := s.storage.GetFullPath(storagePath)
		if profile, err := threemf.Parse(fullPath); err != nil {
			slog.Warn("failed to parse 3MF metadata", "file", filename, "error", err)
		} else if profile != nil {
			design.SliceProfile = profile
		}
	}

	if err := s.repo.Create(ctx, design); err != nil {
		return nil, fmt.Errorf("failed to create design: %w", err)
	}

	return design, nil
}

func (s *DesignService) CreateFromRootFileLibrary(ctx context.Context, partID, fileID uuid.UUID, fileKind string, notes string) (*model.Design, error) {
	switch fileKind {
	case "stl":
		return s.CreateFromSTLLibrary(ctx, partID, fileID, notes)
	default:
		return s.CreateFromGCodeLibrary(ctx, partID, fileID, notes)
	}
}

func (s *DesignService) CreateFromGCodeLibrary(ctx context.Context, partID, gcodeFileID uuid.UUID, notes string) (*model.Design, error) {
	if partID == uuid.Nil {
		return nil, fmt.Errorf("part ID is required")
	}
	if gcodeFileID == uuid.Nil {
		return nil, fmt.Errorf("gcode file ID is required")
	}
	entry, err := s.gcodeRepo.GetByID(ctx, gcodeFileID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("gcode file not found")
	}
	if entry.ParentSTLID != nil {
		return nil, fmt.Errorf("only root files can be added as parts")
	}
	file, err := s.fileRepo.GetByID(ctx, entry.FileID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("file not found")
	}
	design := &model.Design{
		PartID:        partID,
		FileID:        file.ID,
		FileName:      file.OriginalName,
		FileHash:      file.Hash,
		FileSizeBytes: file.SizeBytes,
		FileType:      model.FileTypeGCODE,
		Notes:         notes,
	}
	if err := s.repo.Create(ctx, design); err != nil {
		return nil, fmt.Errorf("failed to create design: %w", err)
	}
	return design, nil
}

func (s *DesignService) CreateFromSTLLibrary(ctx context.Context, partID, stlFileID uuid.UUID, notes string) (*model.Design, error) {
	if partID == uuid.Nil {
		return nil, fmt.Errorf("part ID is required")
	}
	if stlFileID == uuid.Nil {
		return nil, fmt.Errorf("stl file ID is required")
	}
	entry, err := s.stlRepo.GetByID(ctx, stlFileID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("stl file not found")
	}
	file, err := s.fileRepo.GetByID(ctx, entry.FileID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("file not found")
	}
	design := &model.Design{
		PartID:        partID,
		FileID:        file.ID,
		FileName:      file.OriginalName,
		FileHash:      file.Hash,
		FileSizeBytes: file.SizeBytes,
		FileType:      model.FileTypeSTL,
		Notes:         notes,
	}
	if err := s.repo.Create(ctx, design); err != nil {
		return nil, fmt.Errorf("failed to create design: %w", err)
	}
	_ = s.addProjectTagToSTL(ctx, partID, entry.ID)
	return design, nil
}

func (s *DesignService) addProjectTagToSTL(ctx context.Context, partID, stlID uuid.UUID) error {
	if s.partRepo == nil || s.projectRepo == nil || s.tagRepo == nil || s.stlRepo == nil {
		return nil
	}
	part, err := s.partRepo.GetByID(ctx, partID)
	if err != nil || part == nil {
		return err
	}
	project, err := s.projectRepo.GetByID(ctx, part.ProjectID)
	if err != nil || project == nil {
		return err
	}
	tag, err := s.projectTag(ctx, project.Name)
	if err != nil {
		return err
	}
	return s.stlRepo.AddTagToFile(ctx, stlID, tag.ID)
}

func (s *DesignService) removeProjectTagFromSTLIfUnused(ctx context.Context, design *model.Design) error {
	if s.partRepo == nil || s.projectRepo == nil || s.tagRepo == nil || s.stlRepo == nil || design == nil || design.FileType != model.FileTypeSTL {
		return nil
	}
	part, err := s.partRepo.GetByID(ctx, design.PartID)
	if err != nil || part == nil {
		return err
	}
	project, err := s.projectRepo.GetByID(ctx, part.ProjectID)
	if err != nil || project == nil {
		return err
	}
	stl, err := s.stlRepo.GetByFileID(ctx, design.FileID)
	if err != nil || stl == nil {
		return err
	}
	stillUsed, err := s.repo.ProjectHasFileDesign(ctx, project.ID, design.FileID)
	if err != nil || stillUsed {
		return err
	}
	tag, err := s.tagRepo.GetByName(ctx, projectTagName(project.Name))
	if err != nil || tag == nil {
		return err
	}
	return s.stlRepo.RemoveTagFromFile(ctx, stl.ID, tag.ID)
}

func (s *DesignService) projectTag(ctx context.Context, projectName string) (*model.Tag, error) {
	name := projectTagName(projectName)
	tag, err := s.tagRepo.GetByName(ctx, name)
	if err != nil || tag != nil {
		return tag, err
	}
	tag = &model.Tag{Name: name, Color: "#f59e0b"}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, err
	}
	return tag, nil
}

func projectTagName(projectName string) string {
	name := strings.TrimSpace(projectName)
	if name == "" {
		name = "Projeto"
	}
	return "Projeto: " + name
}

func (s *DesignService) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("design ID is required")
	}
	design, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.removeProjectTagFromSTLIfUnused(ctx, design)
}

// GetByID retrieves a design by ID.
func (s *DesignService) GetByID(ctx context.Context, id uuid.UUID) (*model.Design, error) {
	return s.repo.GetByID(ctx, id)
}

// ListByPart retrieves all designs for a part.
func (s *DesignService) ListByPart(ctx context.Context, partID uuid.UUID) ([]model.Design, error) {
	return s.repo.ListByPart(ctx, partID)
}

// GetFile retrieves the file for a design.
func (s *DesignService) GetFile(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.Design, error) {
	design, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if design == nil {
		return nil, nil, fmt.Errorf("design not found")
	}

	file, err := s.fileRepo.GetByID(ctx, design.FileID)
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

	return reader, design, nil
}

// OpenInExternalApp opens a design file in an external application.
func (s *DesignService) OpenInExternalApp(ctx context.Context, id uuid.UUID, appName string) error {
	design, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if design == nil {
		return fmt.Errorf("design not found")
	}

	file, err := s.fileRepo.GetByID(ctx, design.FileID)
	if err != nil {
		return err
	}
	if file == nil {
		return fmt.Errorf("file not found")
	}

	fullPath := s.storage.GetFullPath(file.StoragePath)
	if _, err := os.Stat(fullPath); err != nil {
		return fmt.Errorf("file not found on disk: %w", err)
	}

	var cmd *exec.Cmd
	if appName != "" {
		cmd = exec.Command("open", "-a", appName, fullPath)
	} else {
		cmd = exec.Command("open", fullPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to open application: %w: %s", err, string(output))
	}

	return nil
}

// PrinterService handles printer business logic.
