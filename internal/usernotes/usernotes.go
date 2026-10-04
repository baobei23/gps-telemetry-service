// Package usernotes maintains notes of a user
package usernotes

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Note struct {
	ID        string
	Title     string
	Content   string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (note *Note) ValidateForCreate() error {
	if note == nil {
		return errors.New("validation: empty note")
	}

	note.Sanitize()
	if note.Title == "" {
		return errors.New("validation: note title cannot be empty")
	}

	if note.Content == "" {
		return errors.New("validation: note content cannot be empty")
	}

	if note.UserID == "" {
		return errors.New("validation: note creator cannot be anonymous")
	}

	return nil
}

func (note *Note) Sanitize() {
	note.Title = strings.TrimSpace(note.Title)
	note.Content = strings.TrimSpace(note.Content)
}

type store interface {
	GetNoteByID(ctx context.Context, userID string, noteID string) (*Note, error)
	SaveNote(ctx context.Context, note *Note) (string, error)
}

type UserNotes struct {
	store store
}

func (un *UserNotes) SaveNote(ctx context.Context, note *Note) (*Note, error) {
	err := note.ValidateForCreate()
	if err != nil {
		return nil, err
	}

	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()
	note.ID, err = un.store.SaveNote(ctx, note)
	if err != nil {
		return nil, err
	}

	return note, nil
}

func (un *UserNotes) GetNoteByID(ctx context.Context, userID string, noteID string) (*Note, error) {
	return un.store.GetNoteByID(ctx, userID, noteID)
}

func NewService(store store) *UserNotes {
	return &UserNotes{
		store: store,
	}
}
