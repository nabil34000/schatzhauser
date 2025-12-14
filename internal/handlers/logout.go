package handlers

import (
	"database/sql"
	"net/http"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/httpx"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type LogoutHandler struct {
	DB     *sql.DB
	Guards []protect.Guard
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	h.logout(w, r)
}

// logout performs the actual session deletion and cookie clearing
func (h *LogoutHandler) logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		// No cookie — idempotent success
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"message": "no session",
		})
		return
	}

	token := c.Value
	store := db.NewStore(h.DB)

	// Best-effort delete from DB
	_ = store.DeleteSessionByToken(r.Context(), token)

	// Clear cookie
	ClearSessionCookie(w, r)

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "logged out",
	})
}
