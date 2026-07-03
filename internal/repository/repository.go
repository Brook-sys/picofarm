package repository

import (
	"context"
	"database/sql"
	"encoding/json"
)

// DBTX is an interface for database operations that works with both *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Repositories holds all repository instances.
type Repositories struct {
	db                   *sql.DB
	Projects             *ProjectRepository
	Parts                *PartRepository
	Designs              *DesignRepository
	Printers             *PrinterRepository
	Materials            *MaterialRepository
	Spools               *SpoolRepository
	PrintJobs            *PrintJobRepository
	Files                *FileRepository
	Expenses             *ExpenseRepository
	Sales *SaleRepository
	Etsy  *EtsyRepository
	Squarespace          *SquarespaceRepository
	BambuCloud           *BambuCloudRepository
	Settings             *SettingsRepository
	ProjectSupplies      *ProjectSupplyRepository
	Dispatch             *DispatchRepository
	AutoDispatchSettings *AutoDispatchSettingsRepository
	PrinterMacros        *PrinterMacroRepository
	// New repositories for feature gaps
	Orders          *OrderRepository
	Tags            *TagRepository
	AlertDismissals *AlertDismissalRepository
	Shopify         *ShopifyRepository
	Tasks           *TaskRepository
	TaskChecklist   *TaskChecklistRepository
	Feedback        *FeedbackRepository
	Customers       *CustomerRepository
	Quotes          *QuoteRepository
	Cameras         *CameraRepository
	Timelapses      *TimelapseRepository
	PrintArchives   *PrintArchiveRepository
	QueueItems      *QueueItemRepository
	GCodeLibrary    *GCodeLibraryRepository
	STLLibrary      *STLLibraryRepository
	Notifications   *NotificationRepository
}

// WithTransaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func (r *Repositories) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}

	return tx.Commit()
}

// NewRepositories creates all repository instances.
func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		db:                   db,
		Projects:             &ProjectRepository{db: db},
		Parts:                &PartRepository{db: db},
		Designs:              &DesignRepository{db: db},
		Printers:             &PrinterRepository{db: db},
		Materials:            &MaterialRepository{db: db},
		Spools:               &SpoolRepository{db: db},
		PrintJobs:            &PrintJobRepository{db: db},
		Files:                &FileRepository{db: db},
		Expenses:             &ExpenseRepository{db: db},
		Sales: &SaleRepository{db: db},
		Etsy:  &EtsyRepository{db: db},
		Squarespace:          &SquarespaceRepository{db: db},
		BambuCloud:           &BambuCloudRepository{db: db},
		Settings:             &SettingsRepository{db: db},
		ProjectSupplies:      &ProjectSupplyRepository{db: db},
		Dispatch:             &DispatchRepository{db: db},
		AutoDispatchSettings: &AutoDispatchSettingsRepository{db: db},
		PrinterMacros:        &PrinterMacroRepository{db: db},
		// New repositories for feature gaps
		Orders:          &OrderRepository{db: db},
		Tags:            &TagRepository{db: db},
		AlertDismissals: &AlertDismissalRepository{db: db},
		Shopify:         &ShopifyRepository{db: db},
		Tasks:           &TaskRepository{db: db},
		TaskChecklist:   &TaskChecklistRepository{db: db},
		Feedback:        &FeedbackRepository{db: db},
		Customers:       &CustomerRepository{db: db},
		Quotes:          &QuoteRepository{db: db},
		Cameras:         &CameraRepository{db: db},
		Timelapses:      &TimelapseRepository{db: db},
		PrintArchives:   &PrintArchiveRepository{db: db},
		QueueItems:      &QueueItemRepository{db: db},
		GCodeLibrary:    &GCodeLibraryRepository{db: db},
		STLLibrary:      &STLLibraryRepository{db: db},
		Notifications:   &NotificationRepository{db: db},
	}
}

// marshalStringArray serializes a []string to a JSON string for SQLite TEXT storage.
func marshalStringArray(arr []string) string {
	if arr == nil {
		return "[]"
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

// unmarshalStringArray deserializes a JSON string from SQLite TEXT into []string.
func unmarshalStringArray(data []byte) []string {
	if data == nil {
		return []string{}
	}
	var arr []string
	json.Unmarshal(data, &arr)
	if arr == nil {
		return []string{}
	}
	return arr
}

// ProjectRepository handles project database operations.
