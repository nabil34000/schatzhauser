package handlers

import (
	"database/sql"
	"net/http"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/httpx"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type ProfileHandler struct {
	DB     *sql.DB
	Guards []protect.Guard
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ────────────────────────────────────────
	// Guards enforcement
	// ────────────────────────────────────────
	for _, g := range h.Guards {
		if !g.Check(w, r) {
			return
		}
	}

	h.profile(w, r)
}

// profile contains the main business logic for fetching user info
func (h *ProfileHandler) profile(w http.ResponseWriter, r *http.Request) {
	// Load session from cookie
	sess, err := GetSessionFromRequest(r.Context(), h.DB, r)
	if err != nil {
		httpx.Unauthorized(w, "unauthorized")
		return
	}

	// Load user by ID
	store := db.NewStore(h.DB)
	user, err := store.GetUserByID(r.Context(), sess.UserID)
	if err != nil {
		httpx.Unauthorized(w, "unauthorized")
		return
	}

	// Return safe user info
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"user": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"created":  user.CreatedAt,
		},
	})
}
