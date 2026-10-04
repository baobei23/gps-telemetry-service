package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/baobei23/goapp/internal/usernotes"
)

type RegisterNoteRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// createNote godoc
//
//	@Summary		Create User Note
//	@Description	Create a new note for the authenticated user
//	@Tags			Notes
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RegisterNoteRequest	true	"Note Payload"
//	@Success		201		{object}	BaseResponse{data=RegisterNoteRequest}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/usernotes [post]
//	@Security		ApiKeyAuth
func (h *Handlers) RegisterNote(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	var req RegisterNoteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("title and content are required"))
		return
	}

	unote := &usernotes.Note{
		Title:   req.Title,
		Content: req.Content,
		UserID:  userID,
	}

	un, err := h.notes.SaveNote(r.Context(), unote)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, un)
}

// readUserNote godoc
//
//	@Summary		Read User Note
//	@Description	Read a user note
//	@Tags			Notes
//	@Accept			json
//	@Produce		json
//	@Param			noteID	path		string	true	"Note ID"
//	@Success		200		{object}	BaseResponse{data=usernotes.Note}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/usernotes/{noteID} [get]
//	@Security		ApiKeyAuth
func (h *Handlers) ReadUserNote(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	noteID := r.PathValue("noteID")
	if noteID == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("noteID is required"))
		return
	}

	un, err := h.notes.GetNoteByID(r.Context(), userID, noteID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, un)
}
