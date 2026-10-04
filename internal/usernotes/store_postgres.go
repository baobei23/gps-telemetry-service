package usernotes

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var QueryTimeoutDuration = 5 * time.Second

type pgstore struct {
	pqdriver *pgxpool.Pool
}

func (ps *pgstore) GetNoteByID(ctx context.Context, userID string, noteID string) (*Note, error) {
	query := `
		SELECT title, content, created_at, updated_at
		FROM user_notes
		WHERE id = $1 AND user_id = $2`

	usernote := &Note{
		ID:     noteID,
		UserID: userID,
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := ps.pqdriver.QueryRow(
		ctx, query, noteID, userID,
	).Scan(
		&usernote.Title,
		&usernote.Content,
		&usernote.CreatedAt,
		&usernote.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed getting user note: %w", err)
	}

	return usernote, nil
}

func (ps *pgstore) SaveNote(ctx context.Context, note *Note) (string, error) {
	query := `
		INSERT INTO user_notes (id, title, content, user_id)
		VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var noteID string
	err := ps.pqdriver.QueryRow(ctx, query,
		note.Title,
		note.Content,
		note.UserID,
	).Scan(&noteID)
	if err != nil {
		return "", fmt.Errorf("failed storing note: %w", err)
	}

	return noteID, nil
}

func NewPostgresStore(pqdriver *pgxpool.Pool) store {
	return &pgstore{
		pqdriver: pqdriver,
	}
}
