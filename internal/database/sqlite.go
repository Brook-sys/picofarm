package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DefaultDBPath returns the default database path (~/.picofarm/picofarm.db).
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".picofarm", "picofarm.db"), nil
}

// Open opens or creates a SQLite database at the given path.
// It configures WAL mode, foreign keys, and busy timeout.
func Open(path string) (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Single connection for SQLite to ensure PRAGMAs persist
	db.SetMaxOpenConns(1)

	// Configure connection
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// Auto-backup before migrations (if enabled and not a fresh database)
	CreateStartupBackup(db, path)

	// Run schema
	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("database opened", "path", path)
	return db, nil
}

// RunMigrations applies the embedded schema to the database.
func RunMigrations(db *sql.DB) error {
	if err := validateQueueActiveAssignments(db); err != nil {
		return err
	}

	_, err := db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Add columns that may not exist in older databases.
	// SQLite doesn't support ADD COLUMN IF NOT EXISTS, so we ignore errors.
	alterStatements := []string{
		`ALTER TABLE printers ADD COLUMN serial_number TEXT DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN cost_per_hour_cents INTEGER DEFAULT 0`,
		`ALTER TABLE printers ADD COLUMN purchase_price_cents INTEGER DEFAULT 0`,
		`ALTER TABLE printers ADD COLUMN maintenance_mode BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE printers ADD COLUMN restrict_gcode_model BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE printers ADD COLUMN fluidd_url TEXT DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN default_print_folder TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE material_spools ADD COLUMN default_for_material BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE parts ADD COLUMN material_type TEXT DEFAULT ''`,
		`ALTER TABLE queue_items ADD COLUMN progress REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE queue_items ADD COLUMN wasted_grams REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE queue_items ADD COLUMN failed_attempts INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE queue_items ADD COLUMN filament_name TEXT`,
		`ALTER TABLE queue_items ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL`,
		`CREATE TABLE IF NOT EXISTS stl_files (id TEXT PRIMARY KEY, file_id TEXT NOT NULL REFERENCES files(id), display_name TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_stl_files_created ON stl_files(created_at DESC)`,
		`ALTER TABLE stl_files ADD COLUMN thumbnail_file_id TEXT REFERENCES files(id)`,
		`ALTER TABLE gcode_files ADD COLUMN filament_name TEXT`,
		`ALTER TABLE gcode_files ADD COLUMN parent_stl_id TEXT REFERENCES stl_files(id) ON DELETE SET NULL`,
		`ALTER TABLE gcode_files ADD COLUMN default_for_stl BOOLEAN NOT NULL DEFAULT FALSE`,
		`CREATE INDEX IF NOT EXISTS idx_gcode_files_parent_stl ON gcode_files(parent_stl_id)`,
		`UPDATE gcode_files SET material_type = LOWER(TRIM(SUBSTR(material_type, 1, INSTR(material_type || '#', '#') - 1))) WHERE material_type IS NOT NULL`,
		`UPDATE queue_items SET material_type = LOWER(TRIM(SUBSTR(material_type, 1, INSTR(material_type || '#', '#') - 1))) WHERE material_type IS NOT NULL`,
		`UPDATE gcode_files SET material_color = '' WHERE material_color LIKE '#%'`,
		`UPDATE queue_items SET material_color = '' WHERE material_color LIKE '#%'`,
		`CREATE TABLE IF NOT EXISTS tags (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, color TEXT NOT NULL DEFAULT '#64748b', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`ALTER TABLE tags ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`CREATE TABLE IF NOT EXISTS gcode_file_tags (gcode_file_id TEXT NOT NULL REFERENCES gcode_files(id) ON DELETE CASCADE, tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE, PRIMARY KEY (gcode_file_id, tag_id))`,
		`CREATE INDEX IF NOT EXISTS idx_gcode_file_tags_file ON gcode_file_tags(gcode_file_id)`,
		`CREATE INDEX IF NOT EXISTS idx_gcode_file_tags_tag ON gcode_file_tags(tag_id)`,
		`CREATE TABLE IF NOT EXISTS stl_file_tags (stl_file_id TEXT NOT NULL REFERENCES stl_files(id) ON DELETE CASCADE, tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE, PRIMARY KEY (stl_file_id, tag_id))`,
		`CREATE INDEX IF NOT EXISTS idx_stl_file_tags_file ON stl_file_tags(stl_file_id)`,
		`CREATE INDEX IF NOT EXISTS idx_stl_file_tags_tag ON stl_file_tags(tag_id)`,
		`INSERT OR IGNORE INTO tags (id, name, color, created_at, updated_at)
			SELECT lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6))), 'Projeto: ' || p.name, '#f59e0b', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM projects p
			WHERE EXISTS (SELECT 1 FROM parts pa JOIN designs d ON d.part_id = pa.id JOIN stl_files sf ON sf.file_id = d.file_id WHERE pa.project_id = p.id AND d.file_type = 'stl')
			AND NOT EXISTS (SELECT 1 FROM tags t WHERE t.name = 'Projeto: ' || p.name)`,
		`INSERT OR IGNORE INTO stl_file_tags (stl_file_id, tag_id)
			SELECT sf.id, t.id
			FROM projects p
			JOIN parts pa ON pa.project_id = p.id
			JOIN designs d ON d.part_id = pa.id
			JOIN stl_files sf ON sf.file_id = d.file_id
			JOIN tags t ON t.name = 'Projeto: ' || p.name
			WHERE d.file_type = 'stl'`,
		`ALTER TABLE print_jobs ADD COLUMN printer_time_cost_cents INTEGER`,
		`ALTER TABLE print_jobs ADD COLUMN material_cost_cents INTEGER`,
		`ALTER TABLE project_supplies ADD COLUMN material_id TEXT REFERENCES materials(id)`,
		// Auto-dispatch feature columns
		`ALTER TABLE print_jobs ADD COLUMN priority INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE print_jobs ADD COLUMN auto_dispatch_enabled INTEGER NOT NULL DEFAULT 1`,
		// Low-spool threshold (Phase 1)
		`ALTER TABLE materials ADD COLUMN low_threshold_grams INTEGER NOT NULL DEFAULT 100`,
		// Unified orders (Phase 2) - link projects to orders (legacy, kept for backwards compatibility)
		`ALTER TABLE projects ADD COLUMN order_id TEXT REFERENCES orders(id)`,
		`ALTER TABLE projects ADD COLUMN order_item_id TEXT REFERENCES order_items(id)`,
		// Link Etsy receipts to unified orders
		`ALTER TABLE etsy_receipts ADD COLUMN order_id TEXT REFERENCES orders(id)`,
		// Link Squarespace orders to unified orders
		`ALTER TABLE squarespace_orders ADD COLUMN order_id TEXT REFERENCES orders(id)`,
		// Projects as Product Catalog (template-like fields)
		`ALTER TABLE projects ADD COLUMN sku TEXT`,
		`ALTER TABLE projects ADD COLUMN price_cents INTEGER`,
		`ALTER TABLE projects ADD COLUMN printer_type TEXT`,
		`ALTER TABLE projects ADD COLUMN allowed_printer_ids TEXT DEFAULT '[]'`,
		`ALTER TABLE projects ADD COLUMN default_settings TEXT DEFAULT '{}'`,
		`ALTER TABLE projects ADD COLUMN notes TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN source_url TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN source_provider TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN source_author TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN source_license TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN source_description TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN cover_file_id TEXT REFERENCES files(id)`,
		// Tasks (work instances) - task_id in print_jobs
		`ALTER TABLE print_jobs ADD COLUMN task_id TEXT REFERENCES tasks(id)`,
		// Order items link to projects
		`ALTER TABLE order_items ADD COLUMN project_id TEXT REFERENCES projects(id)`,
		`UPDATE order_items SET project_id = (SELECT p.id FROM projects p WHERE p.template_id = order_items.template_id ORDER BY p.created_at DESC LIMIT 1) WHERE project_id IS NULL AND template_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.template_id = order_items.template_id)`,
		`UPDATE order_items SET project_id = template_id WHERE project_id IS NULL AND template_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = order_items.template_id)`,
		`UPDATE print_jobs SET project_id = (SELECT p.id FROM projects p WHERE p.template_id = print_jobs.recipe_id ORDER BY p.created_at DESC LIMIT 1) WHERE project_id IS NULL AND recipe_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.template_id = print_jobs.recipe_id)`,
		`UPDATE print_jobs SET project_id = recipe_id WHERE project_id IS NULL AND recipe_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = print_jobs.recipe_id)`,
		`UPDATE queue_items SET project_id = template_id WHERE project_id IS NULL AND template_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = queue_items.template_id)`,
		// Task pickup/shipping date
		`ALTER TABLE tasks ADD COLUMN pickup_date TEXT`,
		// Quote line items link to projects
		`ALTER TABLE quote_line_items ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL`,
		// Sales link to customers
		`ALTER TABLE sales ADD COLUMN customer_id TEXT REFERENCES customers(id) ON DELETE SET NULL`,
		// Customer addresses
		`ALTER TABLE customers ADD COLUMN billing_address_json TEXT`,
		`ALTER TABLE customers ADD COLUMN shipping_address_json TEXT`,
		// Quote financial fields
		`ALTER TABLE quotes ADD COLUMN discount_type TEXT DEFAULT 'none'`,
		`ALTER TABLE quotes ADD COLUMN discount_value INTEGER DEFAULT 0`,
		`ALTER TABLE quotes ADD COLUMN rush_fee_cents INTEGER DEFAULT 0`,
		`ALTER TABLE quotes ADD COLUMN tax_rate INTEGER DEFAULT 0`,
		`ALTER TABLE quotes ADD COLUMN terms TEXT`,
		`ALTER TABLE quotes ADD COLUMN requested_due_date TEXT`,
		`ALTER TABLE quotes ADD COLUMN billing_address_json TEXT`,
		`ALTER TABLE quotes ADD COLUMN shipping_address_json TEXT`,
		`ALTER TABLE quotes ADD COLUMN share_token TEXT`,
		`ALTER TABLE auto_dispatch_settings ADD COLUMN macro_auto_dispatch_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE auto_dispatch_settings ADD COLUMN macro_empty_queue_gcode TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS cameras (id TEXT PRIMARY KEY, printer_id TEXT REFERENCES printers(id) ON DELETE SET NULL, name TEXT NOT NULL, type TEXT NOT NULL DEFAULT 'mjpeg', url TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT TRUE, token TEXT DEFAULT '', token_expires_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_cameras_printer_id ON cameras(printer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cameras_enabled ON cameras(enabled)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cameras_token ON cameras(token) WHERE token != ''`,
		`CREATE TABLE IF NOT EXISTS timelapses (id TEXT PRIMARY KEY, printer_id TEXT REFERENCES printers(id) ON DELETE SET NULL, camera_id TEXT REFERENCES cameras(id) ON DELETE SET NULL, print_job_id TEXT REFERENCES print_jobs(id) ON DELETE SET NULL, status TEXT NOT NULL DEFAULT 'pending', frames_path TEXT DEFAULT '', video_path TEXT DEFAULT '', frame_count INTEGER NOT NULL DEFAULT 0, started_at TEXT, completed_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_timelapses_printer_id ON timelapses(printer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_timelapses_status ON timelapses(status)`,
		`CREATE TRIGGER IF NOT EXISTS trg_queue_items_single_active_insert BEFORE INSERT ON queue_items WHEN NEW.assigned_printer_id IS NOT NULL AND NEW.status IN ('printing', 'paused') AND EXISTS (SELECT 1 FROM queue_items WHERE assigned_printer_id = NEW.assigned_printer_id AND status IN ('printing', 'paused')) BEGIN SELECT RAISE(ABORT, 'printer already has an active queue item'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_queue_items_single_active_update BEFORE UPDATE OF assigned_printer_id, status ON queue_items WHEN NEW.assigned_printer_id IS NOT NULL AND NEW.status IN ('printing', 'paused') AND EXISTS (SELECT 1 FROM queue_items WHERE assigned_printer_id = NEW.assigned_printer_id AND id != NEW.id AND status IN ('printing', 'paused')) BEGIN SELECT RAISE(ABORT, 'printer already has an active queue item'); END`,
		`CREATE TABLE IF NOT EXISTS notification_channels (id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT TRUE, config_json TEXT NOT NULL DEFAULT '{}', events_json TEXT NOT NULL DEFAULT '[]', printer_ids_json TEXT NOT NULL DEFAULT '[]', min_severity TEXT NOT NULL DEFAULT 'info', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON notification_channels(type)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_channels_enabled ON notification_channels(enabled)`,
		`CREATE TABLE IF NOT EXISTS notification_deliveries (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE, event_type TEXT NOT NULL, severity TEXT NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT DEFAULT '', payload_json TEXT NOT NULL DEFAULT '{}', sent_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_deliveries_channel ON notification_deliveries(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_deliveries_created ON notification_deliveries(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS notification_templates (id TEXT PRIMARY KEY, channel_id TEXT REFERENCES notification_channels(id) ON DELETE CASCADE, event_type TEXT NOT NULL, format TEXT NOT NULL DEFAULT 'text', title_template TEXT NOT NULL DEFAULT '', body_template TEXT NOT NULL DEFAULT '', payload_template TEXT NOT NULL DEFAULT '', enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(channel_id, event_type))`,
		`CREATE INDEX IF NOT EXISTS idx_notification_templates_channel ON notification_templates(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_templates_event ON notification_templates(event_type)`,
	}
	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil && !isExpectedCompatibilityMigrationError(stmt, err) {
			return fmt.Errorf("%s: %w", compatibilityMigrationContext(stmt), err)
		}
	}

	// Create indexes that may not exist
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_quotes_share_token ON quotes(share_token)`); err != nil {
		return fmt.Errorf("create unique quote share token index: %w", err)
	}

	return nil
}

func validateQueueActiveAssignments(db *sql.DB) error {
	var tableExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'queue_items'`).Scan(&tableExists); err != nil {
		return fmt.Errorf("inspect queue items table: %w", err)
	}
	if tableExists == 0 {
		return nil
	}

	columns, err := db.Query(`PRAGMA table_info(queue_items)`)
	if err != nil {
		return fmt.Errorf("inspect queue items columns: %w", err)
	}
	defer columns.Close()
	hasStatus := false
	hasPrinter := false
	for columns.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("read queue items columns: %w", err)
		}
		hasStatus = hasStatus || name == "status"
		hasPrinter = hasPrinter || name == "assigned_printer_id"
	}
	if err := columns.Err(); err != nil {
		return fmt.Errorf("read queue items columns: %w", err)
	}
	if !hasStatus || !hasPrinter {
		return nil
	}

	var printerID string
	var activeCount int
	err = db.QueryRow(`
		SELECT assigned_printer_id, COUNT(*)
		FROM queue_items
		WHERE assigned_printer_id IS NOT NULL
		  AND status IN ('printing', 'paused')
		GROUP BY assigned_printer_id
		HAVING COUNT(*) > 1
		LIMIT 1
	`).Scan(&printerID, &activeCount)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("validate active queue items: %w", err)
	}
	return fmt.Errorf("printer %s has %d active queue items; resolve duplicate printing/paused items before startup", printerID, activeCount)
}

func isExpectedCompatibilityMigrationError(stmt string, err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column name") {
		return true
	}
	if strings.Contains(msg, "no such column: template_id") {
		return strings.Contains(stmt, "UPDATE order_items SET project_id") ||
			strings.Contains(stmt, "UPDATE queue_items SET project_id")
	}
	if strings.Contains(msg, "no such column: recipe_id") {
		return strings.Contains(stmt, "UPDATE print_jobs SET project_id")
	}
	return false
}

func compatibilityMigrationContext(stmt string) string {
	switch {
	case strings.Contains(stmt, "UPDATE gcode_files SET material_type"):
		return "normalize gcode file material metadata"
	case strings.Contains(stmt, "UPDATE queue_items SET material_type"):
		return "normalize queue item material metadata"
	case strings.Contains(stmt, "UPDATE order_items SET project_id"):
		return "backfill order item project links"
	case strings.Contains(stmt, "UPDATE print_jobs SET project_id"):
		return "backfill print job project links"
	case strings.Contains(stmt, "UPDATE queue_items SET project_id"):
		return "backfill queue item project links"
	default:
		return "run compatibility migration"
	}
}

// CreateStartupBackup creates an automatic backup before migrations run.
// It reads the backup_auto_on_startup setting directly via raw SQL to avoid
// service layer dependencies. Skips silently on fresh databases (no settings table).
func CreateStartupBackup(db *sql.DB, dbPath string) {
	// Skip in-memory databases (testing)
	if dbPath == ":memory:" || dbPath == "" {
		return
	}

	// Check if settings table exists (fresh databases won't have it)
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='settings'").Scan(&tableName)
	if err != nil {
		return // Fresh database, skip
	}

	// Check if auto-backup on startup is enabled (default: true)
	var value string
	err = db.QueryRow("SELECT value FROM settings WHERE key = 'backup_auto_on_startup'").Scan(&value)
	if err != nil {
		// Setting not found — default is enabled
		value = "true"
	}
	if value != "true" {
		slog.Info("startup backup disabled by setting")
		return
	}

	// Create backup directory
	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		slog.Error("failed to create backup directory for startup backup", "error", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("auto_startup_%s.db", timestamp))

	slog.Info("creating startup backup", "path", backupPath)

	_, err = db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		slog.Error("failed to create startup backup", "error", err)
		return
	}

	slog.Info("startup backup created", "path", backupPath)
}
