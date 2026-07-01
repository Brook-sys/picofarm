package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

type STLLibraryRepository struct {
	db *sql.DB
}

type STLLibraryListOptions struct {
	Query    string
	Sort     string
	Page     int
	PageSize int
}

func (r *STLLibraryRepository) List(ctx context.Context, opts STLLibraryListOptions) ([]model.STLLibraryFile, int, error) {
	args := []any{}
	where := "1=1"
	if opts.Query != "" {
		where += " AND (s.display_name LIKE ? OR f.original_name LIKE ?)"
		args = append(args, "%"+opts.Query+"%", "%"+opts.Query+"%")
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stl_files s JOIN files f ON f.id = s.file_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy := "s.created_at DESC"
	switch opts.Sort {
	case "created_asc":
		orderBy = "s.created_at ASC"
	case "name_asc":
		orderBy = "s.display_name ASC"
	case "name_desc":
		orderBy = "s.display_name DESC"
	case "size_asc":
		orderBy = "f.size_bytes ASC"
	case "size_desc":
		orderBy = "f.size_bytes DESC"
	}
	limit := opts.PageSize
	offset := (opts.Page - 1) * opts.PageSize
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.file_id, s.display_name, f.original_name, f.size_bytes, s.thumbnail_file_id, s.created_at, s.updated_at
		FROM stl_files s JOIN files f ON f.id = s.file_id
		WHERE `+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	res := []model.STLLibraryFile{}
	for rows.Next() {
		var f model.STLLibraryFile
		if err := scanRow(rows, &f.ID, &f.FileID, &f.DisplayName, &f.FileName, &f.SizeBytes, &f.ThumbnailFileID, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, err
		}
		res = append(res, f)
	}
	if tagsByFile, err := r.listTagsForFiles(ctx, res); err == nil {
		for i := range res {
			res[i].Tags = tagsByFile[res[i].ID]
		}
	}
	return res, total, nil
}

func (r *STLLibraryRepository) Create(ctx context.Context, f *model.STLLibraryFile) error {
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	f.UpdatedAt = f.CreatedAt
	_, err := r.db.ExecContext(ctx, `INSERT INTO stl_files (id, file_id, display_name, thumbnail_file_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, f.ID, f.FileID, f.DisplayName, f.ThumbnailFileID, f.CreatedAt, f.UpdatedAt)
	return err
}

func (r *STLLibraryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.STLLibraryFile, error) {
	var f model.STLLibraryFile
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT s.id, s.file_id, s.display_name, f.original_name, f.size_bytes, s.thumbnail_file_id, s.created_at, s.updated_at
		FROM stl_files s JOIN files f ON f.id = s.file_id WHERE s.id = ?
	`, id), &f.ID, &f.FileID, &f.DisplayName, &f.FileName, &f.SizeBytes, &f.ThumbnailFileID, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tags, err := r.ListTagsForFile(ctx, f.ID); err == nil {
		f.Tags = tags
	}
	return &f, nil
}

func (r *STLLibraryRepository) Update(ctx context.Context, f *model.STLLibraryFile) error {
	f.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE stl_files SET display_name = ?, thumbnail_file_id = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(f.DisplayName), f.ThumbnailFileID, f.UpdatedAt, f.ID)
	return err
}

func (r *STLLibraryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM stl_files WHERE id = ?`, id)
	return err
}

func (r *STLLibraryRepository) ListTagsForFile(ctx context.Context, fileID uuid.UUID) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN stl_file_tags sft ON sft.tag_id = t.id
		WHERE sft.stl_file_id = ?
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
	return tags, rows.Err()
}

func (r *STLLibraryRepository) listTagsForFiles(ctx context.Context, files []model.STLLibraryFile) (map[uuid.UUID][]model.Tag, error) {
	if len(files) == 0 {
		return map[uuid.UUID][]model.Tag{}, nil
	}
	ids := make([]any, len(files))
	placeholders := make([]string, len(files))
	for i, f := range files {
		ids[i] = f.ID
		placeholders[i] = "?"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT sft.stl_file_id, t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN stl_file_tags sft ON sft.tag_id = t.id
		WHERE sft.stl_file_id IN (`+joinPlaceholders(placeholders)+`)
		ORDER BY t.name
	`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[uuid.UUID][]model.Tag{}
	for rows.Next() {
		var fileID uuid.UUID
		var t model.Tag
		if err := scanRow(rows, &fileID, &t.ID, &t.Name, &t.Color, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		res[fileID] = append(res[fileID], t)
	}
	return res, rows.Err()
}

func (r *STLLibraryRepository) AddTagToFile(ctx context.Context, fileID, tagID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO stl_file_tags (stl_file_id, tag_id) VALUES (?, ?)`, fileID, tagID)
	return err
}

func (r *STLLibraryRepository) RemoveTagFromFile(ctx context.Context, fileID, tagID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM stl_file_tags WHERE stl_file_id = ? AND tag_id = ?`, fileID, tagID)
	return err
}
