package postgres

import (
	"context"
	"database/sql"

	"enterprise-rag/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = pgx.ErrNoRows

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	var email sql.NullString
	err := r.db.QueryRow(
		ctx,
		`SELECT id::text, username, email, password_hash, created_at, updated_at
		 FROM users
		 WHERE username = $1 AND deleted_at IS NULL`,
		username,
	).Scan(&user.ID, &user.Username, &email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.Email = email.String
	return &user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User
	var email sql.NullString
	err := r.db.QueryRow(
		ctx,
		`SELECT id::text, username, email, password_hash, created_at, updated_at
		 FROM users
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&user.ID, &user.Username, &email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.Email = email.String
	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)`,
		user.ID,
		user.Username,
		sql.NullString{String: user.Email, Valid: user.Email != ""},
		user.PasswordHash,
		user.CreatedAt,
	)
	return err
}
