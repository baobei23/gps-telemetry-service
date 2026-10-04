package http

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/baobei23/goapp/internal/users"
)

type RegisterRequest struct {
	FullName string `json:"fullName" binding:"required,max=255"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8"`
}

func (req *RegisterRequest) validate() error {
	if strings.TrimSpace(req.FullName) == "" || len(req.FullName) > 255 {
		return errors.New("fullName is required and must be at most 255 characters")
	}
	if strings.TrimSpace(req.Email) == "" || len(req.Email) > 255 {
		return errors.New("email is required and must be at most 255 characters")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return errors.New("invalid email address")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// register godoc
//
//	@Summary		Register a new user
//	@Description	Register a new user
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RegisterRequest	true	"Register Payload"
//	@Success		201		{object}	BaseResponse{data=users.User}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/register [post]
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	u := &users.User{
		FullName: req.FullName,
		Email:    req.Email,
		Password: []byte(req.Password),
	}

	createdUser, err := h.users.Register(r.Context(), u)
	if err != nil {
		if errors.Is(err, users.ErrUserEmailAlreadyExists) {
			writeError(w, r, http.StatusConflict, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, createdUser)
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (req *LoginRequest) validate() error {
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return errors.New("invalid email address")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

type LoginResponse struct {
	AccessToken string      `json:"accessToken"`
	User        *users.User `json:"user"`
}

// login godoc
//
//	@Summary		Login
//	@Description	Login
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		LoginRequest	true	"Login Payload"
//	@Success		200		{object}	BaseResponse{data=LoginResponse}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/login [post]
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	user, err := h.users.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	accessToken, refreshToken, jti, err := h.tm.GeneratePair(user.ID, user.Email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	err = h.users.SaveRefreshToken(r.Context(), jti, user.ID, time.Now().Add(h.tm.GetRefreshExpiry()))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   int(h.tm.GetRefreshExpiry().Seconds()),
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, &LoginResponse{
		AccessToken: accessToken,
		User:        user,
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"accessToken"`
}

// refreshToken godoc
//
//	@Summary		Refresh Access Token
//	@Description	Use valid refresh token to get new access token pair
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RefreshTokenRequest	true	"Refresh Token Payload"
//	@Success		200		{object}	BaseResponse{data=RefreshTokenResponse}
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/auth/refresh [post]
func (h *Handlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	_ = decodeJSON(w, r, &req)

	token := req.RefreshToken
	if token == "" {
		if cookie, err := r.Cookie("refreshToken"); err == nil {
			token = cookie.Value
		}
	}
	if token == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("refresh token required"))
		return
	}

	claims, err := h.tm.Validate(token)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	if claims.TokenType != "refresh" {
		writeError(w, r, http.StatusUnauthorized, errors.New("invalid token type"))
		return
	}

	exists, err := h.users.CheckRefreshToken(r.Context(), claims.ID)
	if err != nil || !exists {
		writeError(w, r, http.StatusUnauthorized, errors.New("refresh token invalid or revoked"))
		return
	}

	_ = h.users.RevokeRefreshToken(r.Context(), claims.ID)

	accessToken, refreshToken, newJti, err := h.tm.GeneratePair(claims.UserID, claims.Email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	err = h.users.SaveRefreshToken(r.Context(), newJti, claims.UserID, time.Now().Add(h.tm.GetRefreshExpiry()))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   int(h.tm.GetRefreshExpiry().Seconds()),
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, &RefreshTokenResponse{
		AccessToken: accessToken,
	})
}

// logout godoc
//
//	@Summary		Logout
//	@Description	Logout by revoking refresh token
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RefreshTokenRequest	true	"Refresh Token Payload"
//	@Success		200		{object}	BaseResponse{data=string}
//	@Failure		400		{object}	ErrorResponse
//	@Router			/auth/logout [post]
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	_ = decodeJSON(w, r, &req)

	token := req.RefreshToken
	if token == "" {
		if cookie, err := r.Cookie("refreshToken"); err == nil {
			token = cookie.Value
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	loggedOut := "logged out"

	if token == "" {
		writeJSON(w, http.StatusOK, loggedOut)
		return
	}

	claims, err := h.tm.Validate(token)
	if err != nil || claims.TokenType != "refresh" {
		// Ignore validation errors on logout
		writeJSON(w, http.StatusOK, loggedOut)
		return
	}

	_ = h.users.RevokeRefreshToken(r.Context(), claims.ID)

	writeJSON(w, http.StatusOK, loggedOut)
}
