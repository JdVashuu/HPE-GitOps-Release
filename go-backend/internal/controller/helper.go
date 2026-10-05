package controller

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func readJSON(r *http.Request, target interface{}) error {
	return json.NewDecoder(r.Body).Decode(target)
}

func queryParam(r *http.Request, name, defaultVal string) string {
	val := r.URL.Query().Get(name)
	if strings.TrimSpace(val) == "" {
		return defaultVal
	}
	return strings.TrimSpace(val)
}

func pathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}
