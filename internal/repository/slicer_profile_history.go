package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SlicerProfileRevision is an immutable recovery snapshot, not proof that Orca
// accepted or can slice the profile. Source is hashed to avoid disclosing URLs.
type SlicerProfileRevision struct {
	ID        int64  `json:"id"`
	Hash      string `json:"hash"`
	JSON      string `json:"json"`
	CreatedAt string `json:"created_at"`
}

func (r *SettingsRepository) AppendSlicerRevision(ctx context.Context, source, category, name, raw string) error {
	return r.AppendSlicerRevisions(ctx, source, category, name, raw)
}

// AppendSlicerRevisions commits the complete recovery set or nothing. A quota
// refusal, cancellation or insert failure cannot consume a partial preimage.
func (r *SettingsRepository) AppendSlicerRevisions(ctx context.Context, source, category, name string, snapshots ...string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, raw := range snapshots {
		digest := sha256.Sum256([]byte(raw))
		if len(raw) > 1<<20 {
			return fmt.Errorf("recovery snapshot exceeds 1 MiB")
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO slicer_profile_history(source_hash,category,name,hash,json) SELECT ?,?,?,?,? WHERE (SELECT COALESCE(SUM(length(CAST(json AS BLOB))),0) FROM slicer_profile_history) + ? <= 67108864`, source, category, name, hex.EncodeToString(digest[:]), raw, len(raw))
		if err != nil {
			return fmt.Errorf("persist profile recovery snapshot: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("profile recovery storage quota (64 MiB) reached; save refused without deleting snapshots")
		}
	}
	return tx.Commit()
}

func (r *SettingsRepository) ListSlicerRevisions(ctx context.Context, source, category, name string) ([]SlicerProfileRevision, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,hash,json,created_at FROM slicer_profile_history WHERE source_hash=? AND category=? AND name=? ORDER BY id DESC LIMIT 50`, source, category, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SlicerProfileRevision{}
	// Bound the decoded response as well as the row count. Retained snapshots
	// remain immutable; older/larger entries are not deleted by this read.
	bytes := 0
	for rows.Next() {
		var v SlicerProfileRevision
		if err := rows.Scan(&v.ID, &v.Hash, &v.JSON, &v.CreatedAt); err != nil {
			return nil, err
		}
		if bytes+len(v.JSON) > 4<<20 {
			break
		}
		bytes += len(v.JSON)
		result = append(result, v)
	}
	return result, rows.Err()
}
