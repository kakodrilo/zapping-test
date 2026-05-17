package handler

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	ctxUserID contextKey = "userID"
	ctxEmail  contextKey = "email"
)

type tokenValidator interface {
	Validate(token string) (userID int, email string, err error)
}

func AuthMiddleware(tv tokenValidator, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			jsonErr(w, "missing or invalid authorization header", http.StatusUnauthorized)
			return
		}
		userID, email, err := tv.Validate(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			jsonErr(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxEmail, email)
		next(w, r.WithContext(ctx))
	}
}
