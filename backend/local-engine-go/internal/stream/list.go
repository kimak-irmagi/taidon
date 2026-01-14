package stream

import (
	"encoding/json"
	"net/http"
	"strings"
)

func WantsNDJSON(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "application/x-ndjson")
}

func WriteList[T any](w http.ResponseWriter, r *http.Request, items []T) error {
	if WantsNDJSON(r) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		for _, item := range items {
			if err := enc.Encode(item); err != nil {
				return err
			}
		}
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(items)
}
