package printer

import (
	"context"
	"io"

	"github.com/Brook-sys/picofarm/internal/model"
)

type FileClient interface {
	ListFiles(ctx context.Context, path string) ([]model.PrinterFileEntry, error)
	UploadFile(ctx context.Context, dir string, filename string, file io.Reader) error
	DeleteFile(ctx context.Context, path string) error
	CreateDirectory(ctx context.Context, path string) error
	RenameFile(ctx context.Context, oldPath string, newPath string) error
	MoveFile(ctx context.Context, sourcePath string, destPath string) error
	DownloadFile(ctx context.Context, path string) (io.ReadCloser, error)
	GetFileMetadata(ctx context.Context, path string) (*model.PrinterFileMetadata, error)
	DownloadThumbnail(ctx context.Context, path string) (io.ReadCloser, error)
	StartPrint(ctx context.Context, path string) error
}
