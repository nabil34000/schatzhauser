package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type LoginHandler struct {
	DB               *sql.DB
	IPRLimiter       *protect.IPRateLimiter
	RBodySizeLimiter *protect.RBodySizeLimiter
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ---- IP rate limiting
	if h.IPRLimiter.Enable {
		ip := protect.GetIP(r)
		if ip != "" && !h.IPRLimiter.Allow(ip) {
			tooManyRequests(w)
			return
		}
	}

	//Body size limit is header-based, so this must precede json body decoding
	if h.RBodySizeLimiter != nil {
		if r.ContentLength > h.RBodySizeLimiter.MaxBytes {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}

		if err := h.RBodySizeLimiter.Apply(w, r); err != nil {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	// ---- Decode JSON
	var in LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		if protect.IsPayloadTooLarge(err) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		badRequest(w, "invalid json")
		return
	}

	if in.Username == "" || in.Password == "" {
		badRequest(w, "username and password required")
		return
	}

	store := db.NewStore(h.DB)

	user, err := store.GetUserByUsername(r.Context(), in.Username)
	if err != nil {
		unauthorized(w, "invalid credentials")
		return
	}

	if !comparePassword(user.PasswordHash, in.Password) {
		unauthorized(w, "invalid credentials")
		return
	}

	// ---- Create session
	token, err := generateSessionToken()
	if err != nil {
		internalError(w, "token generation failed")
		return
	}

	expires := time.Now().Add(SessionDuration)

	createdSess, err := store.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:       user.ID,
		SessionToken: token,
		ExpiresAt:    expires,
	})
	if err != nil {
		internalError(w, "cannot create session")
		return
	}

	setSessionCookie(w, r, createdSess.SessionToken, createdSess.ExpiresAt)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"message":  "logged in",
		"username": user.Username,
	})
}
