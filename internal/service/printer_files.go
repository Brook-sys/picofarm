package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type PrinterFileService struct {
	printerRepo *repository.PrinterRepository
}

func NewPrinterFileService(printerRepo *repository.PrinterRepository) *PrinterFileService {
	return &PrinterFileService{printerRepo: printerRepo}
}

func (s *PrinterFileService) List(ctx context.Context, printerID uuid.UUID, dir string) (*model.PrinterFileList, error) {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return nil, err
	}
	files, err := client.ListFiles(ctx, cleanPrinterPath(dir))
	if err != nil {
		return nil, err
	}
	return &model.PrinterFileList{Path: cleanPrinterPath(dir), Files: files}, nil
}

func (s *PrinterFileService) Upload(ctx context.Context, printerID uuid.UUID, dir string, file multipart.File, header *multipart.FileHeader) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	defer file.Close()
	return client.UploadFile(ctx, cleanPrinterPath(dir), header.Filename, file)
}

func (s *PrinterFileService) UploadReader(ctx context.Context, printerID uuid.UUID, dir string, filename string, reader io.Reader) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.UploadFile(ctx, cleanPrinterPath(dir), filename, reader)
}

func (s *PrinterFileService) Delete(ctx context.Context, printerID uuid.UUID, filePath string) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.DeleteFile(ctx, cleanPrinterPath(filePath))
}

func (s *PrinterFileService) CreateDirectory(ctx context.Context, printerID uuid.UUID, dirPath string) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.CreateDirectory(ctx, cleanPrinterPath(dirPath))
}

func (s *PrinterFileService) Rename(ctx context.Context, printerID uuid.UUID, oldPath string, newPath string) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.RenameFile(ctx, cleanPrinterPath(oldPath), cleanPrinterPath(newPath))
}

func (s *PrinterFileService) Move(ctx context.Context, printerID uuid.UUID, sourcePath string, destPath string) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.MoveFile(ctx, cleanPrinterPath(sourcePath), cleanPrinterPath(destPath))
}

func (s *PrinterFileService) Download(ctx context.Context, printerID uuid.UUID, filePath string) (io.ReadCloser, error) {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return nil, err
	}
	return client.DownloadFile(ctx, cleanPrinterPath(filePath))
}

func (s *PrinterFileService) Metadata(ctx context.Context, printerID uuid.UUID, filePath string) (*model.PrinterFileMetadata, error) {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return nil, err
	}
	return client.GetFileMetadata(ctx, cleanPrinterPath(filePath))
}

func (s *PrinterFileService) Thumbnail(ctx context.Context, printerID uuid.UUID, thumbPath string) (io.ReadCloser, error) {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return nil, err
	}
	return client.DownloadThumbnail(ctx, cleanPrinterPath(thumbPath))
}

func (s *PrinterFileService) StartPrint(ctx context.Context, printerID uuid.UUID, filePath string) error {
	client, err := s.client(ctx, printerID)
	if err != nil {
		return err
	}
	return client.StartPrint(ctx, cleanPrinterPath(filePath))
}

func (s *PrinterFileService) client(ctx context.Context, printerID uuid.UUID) (printer.FileClient, error) {
	p, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("printer not found")
	}
	if p.ConnectionType != model.ConnectionTypeMoonraker {
		return nil, fmt.Errorf("printer file management is only available for Moonraker printers")
	}
	return printer.NewMoonrakerClient(p.ID, p.ConnectionURI), nil
}

func cleanPrinterPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	if value == "" || value == "." {
		return ""
	}
	return path.Clean(value)
}

var _ io.Reader
