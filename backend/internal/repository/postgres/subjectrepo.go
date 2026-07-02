package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"enterprise-rag/backend/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSubjectNotFound = errors.New("subject not found")

type SubjectRepo struct {
	db *pgxpool.Pool
}

func NewSubjectRepo(db *pgxpool.Pool) *SubjectRepo {
	return &SubjectRepo{db: db}
}

func (r *SubjectRepo) Create(ctx context.Context, subject *model.Subject) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO subjects (id, name, description, owner_id, visibility, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		subject.ID,
		subject.Name,
		sql.NullString{String: subject.Description, Valid: subject.Description != ""},
		subject.OwnerID,
		subject.Visibility,
		subject.CreatedAt,
	)
	return err
}

func (r *SubjectRepo) GetAccessibleByID(ctx context.Context, subjectID, userID string) (*model.Subject, error) {
	var (
		subject     model.Subject
		description sql.NullString
	)
	err := r.db.QueryRow(
		ctx,
		`SELECT id::text, name, description, owner_id::text, visibility, created_at, updated_at
		 FROM subjects
		 WHERE id = $1
		   AND deleted_at IS NULL
		   AND (owner_id = $2 OR visibility = 'public')`,
		subjectID,
		userID,
	).Scan(
		&subject.ID,
		&subject.Name,
		&description,
		&subject.OwnerID,
		&subject.Visibility,
		&subject.CreatedAt,
		&subject.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	subject.Description = description.String
	return &subject, nil
}

func (r *SubjectRepo) ListAccessible(ctx context.Context, filter model.SubjectListFilter) ([]model.Subject, int64, error) {
	conditions := []string{"deleted_at IS NULL", "(owner_id = $1 OR visibility = 'public')"}
	args := []any{filter.UserID}
	if filter.Keyword != "" {
		args = append(args, "%"+filter.Keyword+"%")
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM subjects WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, filter.Offset)
	rows, err := r.db.Query(
		ctx,
		fmt.Sprintf(`SELECT id::text, name, description, owner_id::text, visibility, created_at, updated_at
			FROM subjects
			WHERE %s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	subjects := make([]model.Subject, 0)
	for rows.Next() {
		var (
			subject     model.Subject
			description sql.NullString
		)
		if err := rows.Scan(
			&subject.ID,
			&subject.Name,
			&description,
			&subject.OwnerID,
			&subject.Visibility,
			&subject.CreatedAt,
			&subject.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		subject.Description = description.String
		subjects = append(subjects, subject)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	return subjects, total, nil
}

func (r *SubjectRepo) ExistsAccessible(ctx context.Context, subjectID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		ctx,
		`SELECT EXISTS (
			SELECT 1 FROM subjects
			WHERE id = $1
			  AND deleted_at IS NULL
			  AND (owner_id = $2 OR visibility = 'public')
		)`,
		subjectID,
		userID,
	).Scan(&exists)
	return exists, err
}

func (r *SubjectRepo) UpdateByOwner(ctx context.Context, subject *model.Subject) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE subjects
		 SET name = $3, description = $4, visibility = $5, updated_at = $6
		 WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		subject.ID,
		subject.OwnerID,
		subject.Name,
		sql.NullString{String: subject.Description, Valid: subject.Description != ""},
		subject.Visibility,
		subject.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSubjectNotFound
	}
	return nil
}

func (r *SubjectRepo) SoftDeleteByOwner(ctx context.Context, subjectID, userID string) (bool, error) {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE subjects
		 SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		subjectID,
		userID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
