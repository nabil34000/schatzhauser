package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/httpx"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type LoginHandler struct {
	DB     *sql.DB
	Guards []protect.Guard
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ServeHTTP handles the HTTP request, applies guards, decodes JSON, and delegates to login().
func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	for _, g := range h.Guards {
		if !g.Check(w, r) {
			return
		}
	}

	var in LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.BadRequest(w, "invalid json")
		return
	}

	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		httpx.BadRequest(w, "username and password required")
		return
	}

	h.login(w, r, in)
}

// login contains the actual business logic of authentication and session creation.
func (h *LoginHandler) login(w http.ResponseWriter, r *http.Request, in LoginInput) {
	store := db.NewStore(h.DB)

	// Fetch user by username
	user, err := store.GetUserByUsername(r.Context(), in.Username)
	if err != nil {
		httpx.Unauthorized(w, "invalid credentials")
		return
	}

	// Verify password
	if !ComparePassword(user.PasswordHash, in.Password) {
		httpx.Unauthorized(w, "invalid credentials")
		return
	}

	// Generate session token
	token, err := GenerateSessionToken()
	if err != nil {
		httpx.InternalError(w, "token generation failed")
		return
	}

	// Set expiry
	expires := time.Now().Add(SessionDuration)

	// Create session in DB
	session, err := store.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:       user.ID,
		SessionToken: token,
		ExpiresAt:    expires,
	})
	if err != nil {
		httpx.InternalError(w, "cannot create session")
		return
	}

	// Set session cookie
	SetSessionCookie(w, r, session.SessionToken, session.ExpiresAt)

	// Return success
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"message":  "logged in",
		"username": user.Username,
	})
}
