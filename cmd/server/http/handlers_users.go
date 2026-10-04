package http

import (
	"errors"
	"net/http"
)

// readUserByID godoc
//
//	@Summary		Read User By ID
//	@Description	Read User By ID
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	BaseResponse{data=users.User}
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/users [get]
//
//	@security		ApiKeyAuth
func (h *Handlers) ReadUserByID(w http.ResponseWriter, r *http.Request) {
	id := GetUserID(r)
	if id == "" {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	out, err := h.users.ReadByID(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, out)
}

// changePassword godoc
//
//	@Summary		Change Password
//	@Description	Change Password
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{old_password=string,new_password=string}	true	"Passwords"
//	@Success		200		{object}	BaseResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/users/password [put]
//
//	@security		ApiKeyAuth
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id := GetUserID(r)
	if id == "" {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := h.users.ChangePassword(r.Context(), id, req.OldPassword, req.NewPassword); err != nil {
		if err.Error() == "invalid credentials" {
			writeError(w, r, http.StatusUnauthorized, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
}
