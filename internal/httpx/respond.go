package httpx

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func BadRequest(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusBadRequest, map[string]any{
		"status":  "error",
		"message": msg,
	})
}

func Unauthorized(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusUnauthorized, map[string]any{
		"status":  "error",
		"message": msg,
	})
}

func TooManyRequests(w http.ResponseWriter) {
	WriteJSON(w, http.StatusTooManyRequests, map[string]any{
		"status":  "error",
		"message": "rate limit exceeded",
	})
}

func InternalError(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusInternalServerError, map[string]any{
		"status":  "error",
		"message": msg,
	})
}

func Created(w http.ResponseWriter, v any) {
	WriteJSON(w, http.StatusCreated, v)
}
