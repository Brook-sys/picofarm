package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
)

type PrinterMacroRepository struct {
	db *sql.DB
}

func (r *PrinterMacroRepository) List(ctx context.Context) ([]model.PrinterMacro, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, command, created_at, updated_at FROM printer_macros ORDER BY title COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	macros := []model.PrinterMacro{}
	for rows.Next() {
		var macro model.PrinterMacro
		if err := scanRow(rows, &macro.ID, &macro.Title, &macro.Command, &macro.CreatedAt, &macro.UpdatedAt); err != nil {
			return nil, err
		}
		macros = append(macros, macro)
	}
	return macros, rows.Err()
}

func (r *PrinterMacroRepository) Create(ctx context.Context, macro *model.PrinterMacro) error {
	now := time.Now().UTC()
	macro.CreatedAt = now
	macro.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `INSERT INTO printer_macros (title, command, created_at, updated_at) VALUES (?, ?, ?, ?)`, macro.Title, macro.Command, macro.CreatedAt, macro.UpdatedAt)
	if err != nil {
		return err
	}
	macro.ID, err = res.LastInsertId()
	return err
}

func (r *PrinterMacroRepository) Update(ctx context.Context, macro *model.PrinterMacro) error {
	macro.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `UPDATE printer_macros SET title = ?, command = ?, updated_at = ? WHERE id = ?`, macro.Title, macro.Command, macro.UpdatedAt, macro.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PrinterMacroRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM printer_macros WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
