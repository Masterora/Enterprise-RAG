package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunRepo struct {
	db *pgxpool.Pool
}

func NewRunRepo(db *pgxpool.Pool) *RunRepo {
	return &RunRepo{db: db}
}

func (r *RunRepo) Create(ctx context.Context, run *model.ChatRun) error {
	_, err := r.db.Exec(ctx, `INSERT INTO chat_runs
		(id, tenant_id, user_id, session_id, message_id, subject_id, status, request, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6, $7, $8::jsonb, $9, $9)`,
		run.ID, run.TenantID, run.UserID, run.SessionID, run.MessageID, run.SubjectID,
		run.Status, run.Request, run.CreatedAt)
	return err
}

func (r *RunRepo) GetForUser(ctx context.Context, runID, tenantID, userID string) (*model.ChatRun, error) {
	var run model.ChatRun
	var sessionID, messageID, errorMessage sql.NullString
	var result []byte
	var completedAt sql.NullTime
	err := r.db.QueryRow(ctx, `SELECT id::text, tenant_id::text, user_id::text,
		COALESCE(session_id::text, ''), COALESCE(message_id::text, ''), subject_id::text,
		status, request, result, error_message, cancel_requested, created_at, updated_at, completed_at
		FROM chat_runs WHERE id = $1 AND tenant_id = $2 AND user_id = $3`,
		runID, tenantID, userID).Scan(
		&run.ID, &run.TenantID, &run.UserID, &sessionID, &messageID, &run.SubjectID,
		&run.Status, &run.Request, &result, &errorMessage, &run.CancelRequested,
		&run.CreatedAt, &run.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	run.SessionID = sessionID.String
	run.MessageID = messageID.String
	run.Result = result
	run.ErrorMessage = errorMessage.String
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, nil
}

func (r *RunRepo) MarkRunning(ctx context.Context, runID, tenantID, userID string) error {
	return r.updateStatus(ctx, runID, tenantID, userID, model.RunStatusRunning, "", nil, false)
}

func (r *RunRepo) Complete(ctx context.Context, runID, tenantID, userID string, result []byte) error {
	command, err := r.db.Exec(ctx, `UPDATE chat_runs SET status = $4, result = $5::jsonb,
		error_message = NULL, completed_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status = $6 AND cancel_requested = false`,
		runID, tenantID, userID, model.RunStatusCompleted, nullableJSON(result), model.RunStatusRunning)
	if err != nil {
		return err
	}
	if command.RowsAffected() > 0 {
		return nil
	}
	cancelled, err := r.db.Exec(ctx, `UPDATE chat_runs SET status = $4, error_message = $5,
		completed_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND cancel_requested = true
		  AND status IN ($6, $7)`, runID, tenantID, userID, model.RunStatusCancelled,
		context.Canceled.Error(), model.RunStatusCreated, model.RunStatusRunning)
	if err != nil {
		return err
	}
	if cancelled.RowsAffected() > 0 {
		return context.Canceled
	}
	return pgx.ErrNoRows
}

func (r *RunRepo) Fail(ctx context.Context, runID, tenantID, userID, status, message string) error {
	if status != model.RunStatusCancelled {
		status = model.RunStatusFailed
	}
	return r.updateStatus(ctx, runID, tenantID, userID, status, message, nil, true)
}

func (r *RunRepo) updateStatus(ctx context.Context, runID, tenantID, userID, status, message string, result []byte, terminal bool) error {
	completedAt := any(nil)
	if terminal {
		completedAt = time.Now()
	}
	command, err := r.db.Exec(ctx, `UPDATE chat_runs SET status = $4,
		result = COALESCE($5::jsonb, result), error_message = NULLIF($6, ''),
		completed_at = $7, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status IN ($8, $9)`,
		runID, tenantID, userID, status, nullableJSON(result), message, completedAt,
		model.RunStatusCreated, model.RunStatusRunning)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *RunRepo) RequestCancel(ctx context.Context, runID, tenantID, userID string) (bool, error) {
	command, err := r.db.Exec(ctx, `UPDATE chat_runs SET cancel_requested = true, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status IN ($4, $5)`,
		runID, tenantID, userID, model.RunStatusCreated, model.RunStatusRunning)
	return err == nil && command.RowsAffected() > 0, err
}

func (r *RunRepo) ResetForResume(ctx context.Context, runID, tenantID, userID string) error {
	command, err := r.db.Exec(ctx, `UPDATE chat_runs SET status = $4, result = NULL,
		error_message = NULL, cancel_requested = false, completed_at = NULL, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status IN ($5, $6)`,
		runID, tenantID, userID, model.RunStatusCreated, model.RunStatusFailed, model.RunStatusCancelled)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return errors.New("run is not resumable")
	}
	return nil
}

func (r *RunRepo) AppendEvent(ctx context.Context, event *model.ChatRunEvent) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, event.RunID); err != nil {
		return 0, err
	}
	var sequence int64
	err = tx.QueryRow(ctx, `INSERT INTO chat_run_events (run_id, sequence, tenant_id, event_type, payload)
		SELECT $1, COALESCE(MAX(sequence), 0) + 1, $2, $3, $4::jsonb
		FROM chat_run_events WHERE run_id = $1
		RETURNING sequence`, event.RunID, event.TenantID, event.Type, event.Payload).Scan(&sequence)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return sequence, nil
}

func (r *RunRepo) ListEvents(ctx context.Context, runID, tenantID, userID string, afterSequence int64, limit int) ([]model.ChatRunEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `SELECT e.run_id::text, e.sequence, e.tenant_id::text,
		e.event_type, e.payload, e.created_at
		FROM chat_run_events e
		JOIN chat_runs r ON r.id = e.run_id
		WHERE e.run_id = $1 AND e.tenant_id = $2 AND r.user_id = $3 AND e.sequence > $4
		ORDER BY e.sequence LIMIT $5`, runID, tenantID, userID, afterSequence, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.ChatRunEvent, 0)
	for rows.Next() {
		var event model.ChatRunEvent
		if err := rows.Scan(&event.RunID, &event.Sequence, &event.TenantID, &event.Type, &event.Payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
