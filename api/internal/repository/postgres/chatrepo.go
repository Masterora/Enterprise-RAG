package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepo struct {
	db *pgxpool.Pool
}

func NewChatRepo(db *pgxpool.Pool) *ChatRepo {
	return &ChatRepo{db: db}
}

func (r *ChatRepo) CreateSession(ctx context.Context, session *model.ChatSession) error {
	_, err := r.db.Exec(ctx, `INSERT INTO chat_sessions
		(id, tenant_id, user_id, subject_id, title, llm_provider, llm_model, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, $6, $7, $8, $8)`,
		session.ID, session.TenantID, session.UserID, session.SubjectID, session.Title,
		session.LLMProvider, session.LLMModel, session.CreatedAt)
	return err
}

func (r *ChatRepo) ListSessions(ctx context.Context, userID string) ([]model.ChatSession, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text, user_id::text, COALESCE(subject_id::text, ''),
		COALESCE(title, ''), COALESCE(llm_provider, ''), COALESCE(llm_model, ''),
		created_at, updated_at
		FROM chat_sessions
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]model.ChatSession, 0)
	for rows.Next() {
		var session model.ChatSession
		if err := rows.Scan(&session.ID, &session.UserID, &session.SubjectID, &session.Title,
			&session.LLMProvider, &session.LLMModel, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, err
		}
		session.Messages, err = r.listMessages(ctx, session.ID, userID)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *ChatRepo) listMessages(ctx context.Context, sessionID, userID string) ([]model.ChatMessage, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text, session_id::text, user_id::text,
		content, COALESCE(answer, ''), COALESCE(citations, '[]'::jsonb),
		COALESCE(metadata, '{}'::jsonb), created_at
		FROM chat_messages
		WHERE session_id = $1 AND user_id = $2
		ORDER BY created_at`, sessionID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.ChatMessage, 0)
	for rows.Next() {
		var (
			message   model.ChatMessage
			citations []byte
			metadata  []byte
		)
		if err := rows.Scan(&message.ID, &message.SessionID, &message.UserID, &message.Question,
			&message.Answer, &citations, &metadata, &message.CreatedAt); err != nil {
			return nil, err
		}
		message.Citations = json.RawMessage(citations)
		message.Metadata = json.RawMessage(metadata)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *ChatRepo) UpdateSession(ctx context.Context, sessionID, userID, title string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE chat_sessions SET title = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, sessionID, userID, title)
	return err == nil && tag.RowsAffected() > 0, err
}

func (r *ChatRepo) DeleteSession(ctx context.Context, sessionID, userID string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE chat_sessions SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, sessionID, userID)
	return err == nil && tag.RowsAffected() > 0, err
}

func (r *ChatRepo) SaveTurn(ctx context.Context, session *model.ChatSession, message *model.ChatMessage) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `UPDATE chat_sessions SET subject_id = NULLIF($3, '')::uuid,
		llm_provider = $4, llm_model = $5, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		session.ID, session.UserID, session.SubjectID, session.LLMProvider, session.LLMModel)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	_, err = tx.Exec(ctx, `INSERT INTO chat_messages
			(id, tenant_id, session_id, user_id, role, content, answer, citations, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'turn', $5, $6, $7::jsonb, $8::jsonb, $9)
			ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content, answer = EXCLUDED.answer,
			citations = EXCLUDED.citations, metadata = EXCLUDED.metadata`,
		message.ID, message.TenantID, message.SessionID, message.UserID, message.Question, message.Answer,
		message.Citations, message.Metadata, message.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
