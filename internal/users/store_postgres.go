package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgstore struct {
	pqdriver *pgxpool.Pool
}

func (ps *pgstore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, full_name, email, password
		FROM users
		WHERE email = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	user := new(User)

	row := ps.pqdriver.QueryRow(ctx, query, email)
	err := row.Scan(&user.ID, &user.FullName, &user.Email, &user.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrUserEmailNotFound, email)
		}
		return nil, fmt.Errorf("failed getting user info: %w", err)
	}

	return user, nil
}

func (ps *pgstore) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, full_name, email, password
		FROM users
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	user := new(User)

	row := ps.pqdriver.QueryRow(ctx, query, id)
	err := row.Scan(&user.ID, &user.FullName, &user.Email, &user.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrUserNotFound, id)
		}
		return nil, fmt.Errorf("failed getting user info: %w", err)
	}

	return user, nil
}

func (ps *pgstore) SaveUser(ctx context.Context, user *User) (string, error) {
	query := `
		INSERT INTO users (id, full_name, email, password)
		VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := ps.pqdriver.QueryRow(ctx, query,
		user.FullName,
		user.Email,
		user.Password,
	).Scan(&user.ID)

	if err != nil {
		if strings.Contains(err.Error(), "violates unique constraint \"users_email_key\"") {
			return "", fmt.Errorf("%w: %s", ErrUserEmailAlreadyExists, user.Email)
		}
		return "", fmt.Errorf("failed storing user info: %w", err)
	}

	return user.ID, nil
}

func (ps *pgstore) BulkSaveUser(ctx context.Context, users []User) error {
	rows := make([][]any, 0, len(users))

	for _, user := range users {
		rows = append(rows, []any{
			user.ID,
			user.FullName,
			user.Email,
			user.Password,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	inserted, err := ps.pqdriver.CopyFrom(
		ctx,
		pgx.Identifier{"users"},
		[]string{"id", "full_name", "email", "password"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("failed inserting users: %w", err)
	}

	ulen := int64(len(users))
	if inserted != ulen {
		return fmt.Errorf(
			"failed inserting %d out of %d users",
			ulen-inserted,
			ulen,
		)
	}

	return nil
}

func (ps *pgstore) UpdatePassword(ctx context.Context, id string, newPassword []byte) error {
	query := `UPDATE users SET password = $1 WHERE id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := ps.pqdriver.Exec(ctx, query, newPassword, id)
	if err != nil {
		return fmt.Errorf("failed updating password: %w", err)
	}

	return nil
}

func (ps *pgstore) SaveRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3)`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := ps.pqdriver.Exec(ctx, query, jti, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("failed saving refresh token: %w", err)
	}
	return nil
}

func (ps *pgstore) CheckRefreshToken(ctx context.Context, jti string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE jti = $1)`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool
	err := ps.pqdriver.QueryRow(ctx, query, jti).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed checking refresh token: %w", err)
	}
	return exists, nil
}

func (ps *pgstore) RevokeRefreshToken(ctx context.Context, jti string) error {
	query := `DELETE FROM refresh_tokens WHERE jti = $1`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := ps.pqdriver.Exec(ctx, query, jti)
	if err != nil {
		return fmt.Errorf("failed revoking refresh token: %w", err)
	}
	return nil
}

func NewPostgresStore(pqdriver *pgxpool.Pool) *pgstore {
	return &pgstore{
		pqdriver: pqdriver,
	}
}
