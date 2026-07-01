package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

type GCodeLibraryRepository struct {
	db *sql.DB
}

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

func (r *GCodeLibraryRepository) List(ctx context.Context, opts GCodeLibraryListOptions) ([]model.GCodeLibraryFile, int, error) {
	args := []any{}
	where := "1=1"
	if opts.Query != "" {
		where += " AND (g.display_name LIKE ? OR f.original_name LIKE ? OR g.material_type LIKE ? OR g.filament_name LIKE ?)"
		args = append(args, "%"+opts.Query+"%", "%"+opts.Query+"%", "%"+opts.Query+"%", "%"+opts.Query+"%")
	}
	if opts.Material != "" {
		where += " AND LOWER(g.material_type) = ?"
		args = append(args, strings.ToLower(opts.Material))
	}
	if opts.Profile != "" {
		where += " AND g.filament_name = ?"
		args = append(args, opts.Profile)
	}
	if opts.Nozzle != "" {
		where += " AND g.nozzle_diameter = ?"
		args = append(args, opts.Nozzle)
	}
	if opts.Layer != "" {
		where += " AND g.layer_height = ?"
		args = append(args, opts.Layer)
	}
	if opts.Usage == "never" {
		where += " AND g.print_count = 0"
	} else if opts.Usage == "printed" {
		where += " AND g.print_count > 0"
	}
	switch opts.TimeBucket {
	case "lt_30":
		where += " AND g.estimated_seconds > 0 AND g.estimated_seconds < 1800"
	case "30_60":
		where += " AND g.estimated_seconds >= 1800 AND g.estimated_seconds < 3600"
	case "1_3h":
		where += " AND g.estimated_seconds >= 3600 AND g.estimated_seconds < 10800"
	case "gt_3h":
		where += " AND g.estimated_seconds >= 10800"
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcode_files g JOIN files f ON f.id = g.file_id WHERE `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy := "g.created_at DESC"
	switch opts.Sort {
	case "created_asc":
		orderBy = "g.created_at ASC"
	case "name_asc":
		orderBy = "g.display_name ASC"
	case "name_desc":
		orderBy = "g.display_name DESC"
	case "prints_desc":
		orderBy = "g.print_count DESC"
	case "time_asc":
		orderBy = "g.estimated_seconds IS NULL, g.estimated_seconds ASC"
	case "grams_asc":
		orderBy = "g.filament_grams IS NULL, g.filament_grams ASC"
	case "grams_desc":
		orderBy = "g.filament_grams DESC"
	}
	limit := opts.PageSize
	offset := (opts.Page - 1) * opts.PageSize
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.file_id, g.display_name, f.original_name, g.material_type, g.material_color, COALESCE(g.filament_name, ''), g.filament_grams, g.estimated_seconds,
		       g.layer_height, g.nozzle_diameter, g.bed_temp, g.nozzle_temp, g.thumbnail_file_id, g.parent_stl_id, g.default_for_stl, g.metadata_json,
		       g.print_count, g.created_at, g.updated_at
		FROM gcode_files g
		JOIN files f ON f.id = g.file_id
		WHERE `+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	res := []model.GCodeLibraryFile{}
	for rows.Next() {
		var f model.GCodeLibraryFile
		var md sql.NullString
		if err := scanRow(rows, &f.ID, &f.FileID, &f.DisplayName, &f.FileName, &f.MaterialType, &f.MaterialColor, &f.FilamentName, &f.FilamentGrams, &f.EstimatedSeconds,
			&f.LayerHeight, &f.NozzleDiameter, &f.BedTemp, &f.NozzleTemp, &f.ThumbnailFileID, &f.ParentSTLID, &f.DefaultForSTL, &md, &f.PrintCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if md.Valid && md.String != "" {
			json.Unmarshal([]byte(md.String), &f.Metadata)
		}
		res = append(res, f)
	}
	if len(res) > 0 {
		if tagsByFile, err := r.listTagsForFiles(ctx, res); err == nil {
			for i := range res {
				res[i].Tags = tagsByFile[res[i].ID]
			}
		}
	}
	return res, total, nil
}

func (r *GCodeLibraryRepository) Create(ctx context.Context, f *model.GCodeLibraryFile) error {
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	f.UpdatedAt = f.CreatedAt
	md := ""
	if f.Metadata != nil {
		b, _ := json.Marshal(f.Metadata)
		md = string(b)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gcode_files (id, file_id, display_name, material_type, material_color, filament_name, filament_grams, estimated_seconds,
			layer_height, nozzle_diameter, bed_temp, nozzle_temp, thumbnail_file_id, parent_stl_id, default_for_stl, metadata_json, print_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.FileID, f.DisplayName, f.MaterialType, f.MaterialColor, f.FilamentName, f.FilamentGrams, f.EstimatedSeconds,
		f.LayerHeight, f.NozzleDiameter, f.BedTemp, f.NozzleTemp, f.ThumbnailFileID, f.ParentSTLID, f.DefaultForSTL, md, f.PrintCount, f.CreatedAt, f.UpdatedAt)
	return err
}

func (r *GCodeLibraryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.GCodeLibraryFile, error) {
	var f model.GCodeLibraryFile
	var md sql.NullString
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT g.id, g.file_id, g.display_name, f.original_name, g.material_type, g.material_color, COALESCE(g.filament_name, ''), g.filament_grams, g.estimated_seconds,
		       g.layer_height, g.nozzle_diameter, g.bed_temp, g.nozzle_temp, g.thumbnail_file_id, g.parent_stl_id, g.default_for_stl, g.metadata_json, g.print_count, g.created_at, g.updated_at
		FROM gcode_files g JOIN files f ON f.id = g.file_id WHERE g.id = ?
	`, id), &f.ID, &f.FileID, &f.DisplayName, &f.FileName, &f.MaterialType, &f.MaterialColor, &f.FilamentName, &f.FilamentGrams, &f.EstimatedSeconds,
		&f.LayerHeight, &f.NozzleDiameter, &f.BedTemp, &f.NozzleTemp, &f.ThumbnailFileID, &f.ParentSTLID, &f.DefaultForSTL, &md, &f.PrintCount, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if md.Valid && md.String != "" {
		json.Unmarshal([]byte(md.String), &f.Metadata)
	}
	if tags, err := r.ListTagsForFile(ctx, f.ID); err == nil {
		f.Tags = tags
	}
	return &f, nil
}

func (r *GCodeLibraryRepository) IncrementPrintCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE gcode_files SET print_count = print_count + 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (r *GCodeLibraryRepository) Update(ctx context.Context, f *model.GCodeLibraryFile) error {
	f.UpdatedAt = time.Now()
	md := ""
	if f.Metadata != nil {
		b, _ := json.Marshal(f.Metadata)
		md = string(b)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE gcode_files SET
			display_name = ?, material_type = ?, material_color = ?, filament_name = ?, filament_grams = ?, estimated_seconds = ?,
			layer_height = ?, nozzle_diameter = ?, bed_temp = ?, nozzle_temp = ?, thumbnail_file_id = ?, parent_stl_id = ?, default_for_stl = ?,
			metadata_json = ?, updated_at = ?
		WHERE id = ?
	`, f.DisplayName, f.MaterialType, f.MaterialColor, f.FilamentName, f.FilamentGrams, f.EstimatedSeconds,
		f.LayerHeight, f.NozzleDiameter, f.BedTemp, f.NozzleTemp, f.ThumbnailFileID, f.ParentSTLID, f.DefaultForSTL, md, f.UpdatedAt, f.ID)
	return err
}

func (r *GCodeLibraryRepository) SetParentSTL(ctx context.Context, id uuid.UUID, parentID *uuid.UUID) error {
	if parentID != nil {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM gcode_file_tags WHERE gcode_file_id = ?`, id); err != nil {
			return err
		}
	}
	now := time.Now()
	defaultForSTL := false
	if parentID != nil {
		var count int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcode_files WHERE parent_stl_id = ?`, *parentID).Scan(&count); err != nil {
			return err
		}
		defaultForSTL = count == 0
	}
	_, err := r.db.ExecContext(ctx, `UPDATE gcode_files SET parent_stl_id = ?, default_for_stl = ?, updated_at = ? WHERE id = ?`, parentID, defaultForSTL, now, id)
	return err
}

func (r *GCodeLibraryRepository) SetDefaultForSTL(ctx context.Context, id uuid.UUID) error {
	entry, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil || entry.ParentSTLID == nil {
		return sql.ErrNoRows
	}
	now := time.Now()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE gcode_files SET default_for_stl = FALSE, updated_at = ? WHERE parent_stl_id = ?`, now, *entry.ParentSTLID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gcode_files SET default_for_stl = TRUE, updated_at = ? WHERE id = ?`, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GCodeLibraryRepository) ClearParentSTL(ctx context.Context, parentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE gcode_files SET parent_stl_id = NULL, default_for_stl = FALSE, updated_at = ? WHERE parent_stl_id = ?`, time.Now(), parentID)
	return err
}

func (r *GCodeLibraryRepository) ListByParentSTL(ctx context.Context, parentID uuid.UUID) ([]model.GCodeLibraryFile, error) {
	items, _, err := r.List(ctx, GCodeLibraryListOptions{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	res := []model.GCodeLibraryFile{}
	for _, item := range items {
		if item.ParentSTLID != nil && *item.ParentSTLID == parentID {
			res = append(res, item)
		}
	}
	return res, nil
}

func (r *GCodeLibraryRepository) ListRoot(ctx context.Context) ([]model.GCodeLibraryFile, error) {
	items, _, err := r.List(ctx, GCodeLibraryListOptions{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	res := []model.GCodeLibraryFile{}
	for _, item := range items {
		if item.ParentSTLID == nil {
			res = append(res, item)
		}
	}
	return res, nil
}

func (r *GCodeLibraryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM gcode_files WHERE id = ?`, id)
	return err
}

func (r *GCodeLibraryRepository) LinkedProjects(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT p.name
		FROM gcode_files g
		JOIN designs d ON d.file_id = g.file_id
		JOIN parts pa ON pa.id = d.part_id
		JOIN projects p ON p.id = pa.project_id
		WHERE g.id = ?
		ORDER BY p.name COLLATE NOCASE
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		projects = append(projects, name)
	}
	return projects, rows.Err()
}

func (r *GCodeLibraryRepository) ListTags(ctx context.Context) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, color, created_at, updated_at FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []model.Tag{}
	for rows.Next() {
		var t model.Tag
		if err := scanRow(rows, &t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (r *GCodeLibraryRepository) ListTagsForFile(ctx context.Context, fileID uuid.UUID) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN gcode_file_tags gft ON gft.tag_id = t.id
		WHERE gft.gcode_file_id = ?
		ORDER BY t.name
	`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []model.Tag{}
	for rows.Next() {
		var t model.Tag
		if err := scanRow(rows, &t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (r *GCodeLibraryRepository) listTagsForFiles(ctx context.Context, files []model.GCodeLibraryFile) (map[uuid.UUID][]model.Tag, error) {
	if len(files) == 0 {
		return map[uuid.UUID][]model.Tag{}, nil
	}
	ids := make([]any, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}
	query := `
		SELECT gft.gcode_file_id, t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN gcode_file_tags gft ON gft.tag_id = t.id
		WHERE gft.gcode_file_id IN (` + joinPlaceholders(placeholders) + `)
		ORDER BY t.name
	`
	rows, err := r.db.QueryContext(ctx, query, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uuid.UUID][]model.Tag)
	for rows.Next() {
		var fid uuid.UUID
		var t model.Tag
		if err := scanRow(rows, &fid, &t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		m[fid] = append(m[fid], t)
	}
	return m, nil
}

func joinPlaceholders(ph []string) string {
	if len(ph) == 0 {
		return ""
	}
	out := ph[0]
	for _, p := range ph[1:] {
		out += "," + p
	}
	return out
}

func (r *GCodeLibraryRepository) CreateTag(ctx context.Context, t *model.Tag) error {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	_, err := r.db.ExecContext(ctx, `INSERT INTO tags (id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, t.ID, t.Name, t.Color, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *GCodeLibraryRepository) DeleteTag(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	return err
}

func (r *GCodeLibraryRepository) AddTagToFile(ctx context.Context, fileID, tagID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO gcode_file_tags (gcode_file_id, tag_id) VALUES (?, ?)`, fileID, tagID)
	return err
}

func (r *GCodeLibraryRepository) RemoveTagFromFile(ctx context.Context, fileID, tagID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM gcode_file_tags WHERE gcode_file_id = ? AND tag_id = ?`, fileID, tagID)
	return err
}
