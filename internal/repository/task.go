package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type TaskRepository struct {
	db *sql.DB
}

// Create inserts a new task.
func (r *TaskRepository) Create(ctx context.Context, t *model.Task) error {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	if t.Status == "" {
		t.Status = model.TaskStatusPending
	}
	if t.Quantity == 0 {
		t.Quantity = 1
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tasks (id, project_id, order_id, order_item_id, name, status, quantity, notes, pickup_date, created_at, updated_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.ProjectID, t.OrderID, t.OrderItemID, t.Name, t.Status, t.Quantity, t.Notes, t.PickupDate, t.CreatedAt, t.UpdatedAt, t.StartedAt, t.CompletedAt)
	return err
}

// GetByID retrieves a task by ID.
func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	var t model.Task
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, project_id, order_id, order_item_id, name, status, quantity, notes, pickup_date, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE id = ?
	`, id), &t.ID, &t.ProjectID, &t.OrderID, &t.OrderItemID, &t.Name, &t.Status, &t.Quantity, &t.Notes, &t.PickupDate, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

// List retrieves tasks with optional filters.
func (r *TaskRepository) List(ctx context.Context, filters model.TaskFilters) ([]model.Task, error) {
	query := `SELECT id, project_id, order_id, order_item_id, name, status, quantity, notes, pickup_date, created_at, updated_at, started_at, completed_at FROM tasks WHERE 1=1`
	args := []interface{}{}

	if filters.ProjectID != nil {
		query += ` AND project_id = ?`
		args = append(args, *filters.ProjectID)
	}
	if filters.OrderID != nil {
		query += ` AND order_id = ?`
		args = append(args, *filters.OrderID)
	}
	if filters.Status != nil {
		query += ` AND status = ?`
		args = append(args, *filters.Status)
	}

	query += ` ORDER BY created_at DESC`

	if filters.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filters.Limit)
	}
	if filters.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := scanRow(rows, &t.ID, &t.ProjectID, &t.OrderID, &t.OrderItemID, &t.Name, &t.Status, &t.Quantity, &t.Notes, &t.PickupDate, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListByProject retrieves all tasks for a project.
func (r *TaskRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.Task, error) {
	return r.List(ctx, model.TaskFilters{ProjectID: &projectID})
}

// ListByOrder retrieves all tasks for an order.
func (r *TaskRepository) ListByOrder(ctx context.Context, orderID uuid.UUID) ([]model.Task, error) {
	return r.List(ctx, model.TaskFilters{OrderID: &orderID})
}

// Update updates a task.
func (r *TaskRepository) Update(ctx context.Context, t *model.Task) error {
	t.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE tasks SET project_id = ?, order_id = ?, order_item_id = ?, name = ?, status = ?, quantity = ?, notes = ?, pickup_date = ?, updated_at = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`, t.ProjectID, t.OrderID, t.OrderItemID, t.Name, t.Status, t.Quantity, t.Notes, t.PickupDate, t.UpdatedAt, t.StartedAt, t.CompletedAt, t.ID)
	return err
}

// TaskChecklistRepository handles task checklist item database operations.
type TaskChecklistRepository struct {
	db *sql.DB
}

// Create inserts a single checklist item.
func (r *TaskChecklistRepository) Create(ctx context.Context, item *model.TaskChecklistItem) error {
	item.ID = uuid.New()
	item.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO task_checklist_items (id, task_id, name, part_id, sort_order, completed, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.TaskID, item.Name, item.PartID, item.SortOrder, item.Completed, item.CompletedAt, item.CreatedAt)
	return err
}

// CreateBatch inserts multiple checklist items.
func (r *TaskChecklistRepository) CreateBatch(ctx context.Context, items []model.TaskChecklistItem) error {
	for i := range items {
		if err := r.Create(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

// ListByTask retrieves all checklist items for a task, ordered by sort_order.
func (r *TaskChecklistRepository) ListByTask(ctx context.Context, taskID uuid.UUID) ([]model.TaskChecklistItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, name, part_id, sort_order, completed, completed_at, created_at
		FROM task_checklist_items WHERE task_id = ? ORDER BY sort_order ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.TaskChecklistItem
	for rows.Next() {
		var item model.TaskChecklistItem
		if err := scanRow(rows, &item.ID, &item.TaskID, &item.Name, &item.PartID, &item.SortOrder, &item.Completed, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateCompleted toggles the completed state of a checklist item.
func (r *TaskChecklistRepository) UpdateCompleted(ctx context.Context, id uuid.UUID, completed bool) error {
	var completedAt interface{}
	if completed {
		now := time.Now()
		completedAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE task_checklist_items SET completed = ?, completed_at = ? WHERE id = ?
	`, completed, completedAt, id)
	return err
}

// DeleteByTask removes all checklist items for a task.
func (r *TaskChecklistRepository) DeleteByTask(ctx context.Context, taskID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM task_checklist_items WHERE task_id = ?`, taskID)
	return err
}

// UpdateStatus updates only the task status and related timestamps.
func (r *TaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus) error {
	now := time.Now()
	var startedAt, completedAt interface{}

	switch status {
	case model.TaskStatusInProgress:
		startedAt = now
	case model.TaskStatusCompleted, model.TaskStatusCancelled:
		completedAt = now
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE tasks SET status = ?, updated_at = ?,
			started_at = COALESCE(?, started_at),
			completed_at = COALESCE(?, completed_at)
		WHERE id = ?
	`, status, now, startedAt, completedAt, id)
	return err
}

// Delete removes a task.
func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// GetProjectTaskStats returns task statistics for a project.
func (r *TaskRepository) GetProjectTaskStats(ctx context.Context, projectID uuid.UUID) (total int, completed int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0)
		FROM tasks WHERE project_id = ?
	`, projectID).Scan(&total, &completed)
	return
}

// GetPendingSalesStats returns the count and estimated revenue of tasks in pending/in_progress status.
// Revenue is estimated from project price_cents if set, otherwise from average gross_cents of past sales.
func (r *TaskRepository) GetPendingSalesStats(ctx context.Context) (count int, revenueCents int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(t.quantity), 0),
			COALESCE(SUM(t.quantity * COALESCE(
				p.price_cents,
				(SELECT CASE WHEN COUNT(*) > 0 THEN SUM(s.gross_cents) / COUNT(*) ELSE 0 END
				 FROM sales s WHERE s.project_id = p.id),
				0
			)), 0)
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.status IN ('pending', 'in_progress')
	`).Scan(&count, &revenueCents)
	return
}

// DesignRepository handles design database operations.
