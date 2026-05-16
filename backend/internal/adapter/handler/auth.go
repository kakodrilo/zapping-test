package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"zapping-test-service/internal/domain"
)

type authUseCase interface {
	Register(ctx context.Context, name, email, password string) error
	Login(ctx context.Context, email, password string) (string, error)
	Refresh(ctx context.Context, userID int, email string) (string, error)
}

type AuthHandler struct {
	uc authUseCase
}

func NewAuthHandler(uc authUseCase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.uc.Register(r.Context(), body.Name, body.Email, body.Password); err != nil {
		switch err {
		case domain.ErrMissingFields:
			jsonErr(w, err.Error(), http.StatusBadRequest)
		case domain.ErrEmailTaken:
			jsonErr(w, err.Error(), http.StatusConflict)
		default:
			jsonErr(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "user created"})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.uc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		jsonErr(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(ctxUserID).(int)
	email := r.Context().Value(ctxEmail).(string)

	token, err := h.uc.Refresh(r.Context(), userID, email)
	if err != nil {
		jsonErr(w, "could not refresh token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
