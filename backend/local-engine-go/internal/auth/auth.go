package auth

import (
	"net/http"
	"strings"
)

func RequireBearer(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "Bearer "+token {
		return true
	}
	w.WriteHeader(http.StatusUnauthorized)
	return false
}
