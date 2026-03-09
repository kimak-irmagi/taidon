package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sqlrs/engine-local/internal/prepare"
	"github.com/sqlrs/engine-local/internal/stream"
)

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(payload)
}

func writeJSONStatus(w http.ResponseWriter, payload any, status int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, payload prepare.ErrorResponse, status int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}

func writeErrorResponse(w http.ResponseWriter, code, message, details string, status int) error {
	return writeError(w, prepare.ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}, status)
}

func readQueryValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func parseBoolQuery(r *http.Request, key string) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func normalizeIDPrefix(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) < 8 {
		return "", fmt.Errorf("id_prefix must be at least 8 hex characters")
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return "", fmt.Errorf("id_prefix must be hex")
	}
	return strings.ToLower(value), nil
}

func writeListResponse[T any](w http.ResponseWriter, r *http.Request, items []T) error {
	return stream.WriteList(w, r, items)
}
