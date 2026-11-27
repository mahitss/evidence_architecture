package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// AdminUserStore queries the admin_users table.
type AdminUserStore struct {
	pool *pgxpool.Pool
}

// GetByEmail retrieves an admin user case-insensitively by email address.
func (as *AdminUserStore) GetByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	const query = `
		SELECT
			id,
			email,
			password_hash,
			role,
			is_active,
			last_login_at,
			created_at,
			updated_at
		FROM admin_users
		WHERE LOWER(email) = LOWER($1)
	`

	var u models.AdminUser
	err := as.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get admin user by email: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get admin user by email: %w", err)
	}
	return &u, nil
}
