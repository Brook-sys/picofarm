package service

import (
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/validation"
)

func TestNormalizePrinterPrintFolder(t *testing.T) {
	t.Parallel()

	printer := &model.Printer{DefaultPrintFolder: "/sda1/"}
	if err := normalizePrinterPrintFolder(printer); err != nil {
		t.Fatalf("normalize valid folder: %v", err)
	}
	if printer.DefaultPrintFolder != "sda1" {
		t.Fatalf("DefaultPrintFolder = %q, want sda1", printer.DefaultPrintFolder)
	}
}

func TestNormalizePrinterPrintFolderReturnsFieldValidationError(t *testing.T) {
	t.Parallel()

	printer := &model.Printer{DefaultPrintFolder: "../config"}
	err := normalizePrinterPrintFolder(printer)
	validationErr, ok := err.(*validation.ValidationError)
	if !ok {
		t.Fatalf("error = %T, want *validation.ValidationError", err)
	}
	if len(validationErr.Errors) != 1 || validationErr.Errors[0].Field != "default_print_folder" {
		t.Fatalf("validation errors = %#v", validationErr.Errors)
	}
}
