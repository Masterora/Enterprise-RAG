package postgres

import (
	"context"
	"database/sql"

	"enterprise-rag/api/internal/model"

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
		`SELECT id::text, tenant_id::text, username, nickname, email, language, password_hash, created_at, updated_at
		 FROM users
		 WHERE username = $1 AND deleted_at IS NULL`,
		username,
	).Scan(&user.ID, &user.TenantID, &user.Username, &user.Nickname, &email, &user.Language, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
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
		`SELECT id::text, tenant_id::text, username, nickname, email, language, password_hash, created_at, updated_at
		 FROM users
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&user.ID, &user.TenantID, &user.Username, &user.Nickname, &email, &user.Language, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.Email = email.String
	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		`INSERT INTO tenants (id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, $3)`,
		user.TenantID,
		user.Nickname,
		user.CreatedAt,
	); err != nil {
		return err
	}
	_, err = tx.Exec(
		ctx,
		`INSERT INTO users (id, tenant_id, username, nickname, email, language, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		user.ID,
		user.TenantID,
		user.Username,
		user.Nickname,
		sql.NullString{String: user.Email, Valid: user.Email != ""},
		user.Language,
		user.PasswordHash,
		user.CreatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *UserRepo) UpdateProfile(ctx context.Context, userID, nickname, email, language string) (*model.User, error) {
	var user model.User
	var storedEmail sql.NullString
	err := r.db.QueryRow(
		ctx,
		`UPDATE users
		 SET nickname = $2,
		     email = $3,
		     language = $4,
		     updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id::text, tenant_id::text, username, nickname, email, language, password_hash, created_at, updated_at`,
		userID,
		nickname,
		sql.NullString{String: email, Valid: email != ""},
		language,
	).Scan(&user.ID, &user.TenantID, &user.Username, &user.Nickname, &storedEmail, &user.Language, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.Email = storedEmail.String
	return &user, nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE users
		 SET password_hash = $2,
		     updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
		passwordHash,
	)
	return err
}
