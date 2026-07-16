package postgres

import (
	"context"
	"encoding/json"

	"enterprise-rag/api/internal/model"
	taskmsg "enterprise-rag/api/internal/task"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type OutboxRepo struct {
	db *pgxpool.Pool
}

func NewOutboxRepo(db *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{db: db}
}

func (r *OutboxRepo) ClaimBatch(ctx context.Context, limit int) ([]model.TaskOutbox, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `UPDATE task_outbox o
		SET locked_at = now(), updated_at = now()
		WHERE o.id IN (
			SELECT id FROM task_outbox
			WHERE published_at IS NULL AND next_attempt_at <= now()
			  AND (locked_at IS NULL OR locked_at < now() - interval '1 minute')
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED LIMIT $1
		)
		RETURNING id::text, task_id::text, subject, payload, headers, attempts, next_attempt_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.TaskOutbox, 0)
	for rows.Next() {
		var item model.TaskOutbox
		var headers []byte
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Subject, &item.Payload, &headers, &item.Attempts, &item.NextAttemptAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(headers, &item.Headers); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OutboxRepo) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE task_outbox SET published_at = now(), locked_at = NULL,
		last_error = NULL, updated_at = now() WHERE id = $1`, id)
	return err
}

func (r *OutboxRepo) MarkFailed(ctx context.Context, id, message string) error {
	_, err := r.db.Exec(ctx, `UPDATE task_outbox SET attempts = attempts + 1,
		next_attempt_at = now() + make_interval(secs => power(2, LEAST(attempts, 6))::int),
		locked_at = NULL, last_error = $2, updated_at = now() WHERE id = $1`, id, message)
	return err
}

func enqueueOutbox(ctx context.Context, tx pgx.Tx, task *model.IndexTask, reset bool) error {
	processingMode := ""
	if task.TaskType == model.TaskTypeParse {
		var metadata taskmsg.ParseTaskMetadata
		if json.Unmarshal(task.Metadata, &metadata) == nil {
			processingMode = taskmsg.NormalizeProcessingMode(metadata.ProcessingMode)
		}
	}
	payload, err := json.Marshal(taskmsg.Message{
		TaskID: task.ID, DocID: task.DocID, ProcessingMode: processingMode,
	})
	if err != nil {
		return err
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("outbox:"+task.ID)).String()
	headers := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
	headerPayload, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	if reset {
		_, err = tx.Exec(ctx, `INSERT INTO task_outbox (id, task_id, subject, payload, headers)
			VALUES ($1, $2, $3, $4::jsonb, $5::jsonb)
			ON CONFLICT (task_id) DO UPDATE SET subject = EXCLUDED.subject,
			payload = EXCLUDED.payload, headers = EXCLUDED.headers, attempts = 0, next_attempt_at = now(), locked_at = NULL,
			published_at = NULL, last_error = NULL, updated_at = now()`, id, task.ID, task.TaskType, payload, headerPayload)
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO task_outbox (id, task_id, subject, payload, headers)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb) ON CONFLICT (task_id) DO NOTHING`, id, task.ID, task.TaskType, payload, headerPayload)
	return err
}
