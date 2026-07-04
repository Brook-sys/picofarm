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
	StartPrint(ctx context.Context, path string) error
}
