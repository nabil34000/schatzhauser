package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/httpx"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type RegisterHandler struct {
	DB *sql.DB

	// Injected guard chain
	Guards []protect.Guard

	// Domain-level limiter config
	AccountPerIPLimiter config.AccountPerIPLimiterConfig
}

type RegisterInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ────────────────────────────────────────
	// Guards
	// ────────────────────────────────────────
	for _, g := range h.Guards {
		if !g.Check(w, r) {
			return
		}
	}

	// ────────────────────────────────────────
	// Input
	// ────────────────────────────────────────
	var in RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.BadRequest(w, "invalid json")
		return
	}

	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		httpx.BadRequest(w, "username and password required")
		return
	}

	h.register(w, r, in)
}

func (h *RegisterHandler) register(
	w http.ResponseWriter,
	r *http.Request,
	in RegisterInput,
) {
	//ip := protect.GetIP(r)
	ip := strings.TrimSpace(protect.GetIP(r))

	store := db.NewStore(h.DB)

	tx, err := h.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		httpx.InternalError(w, "cannot begin transaction")
		return
	}
	defer tx.Rollback()

	txStore := store.WithTx(tx)

	limiter := protect.NewAccountPerIPLimiter(
		h.AccountPerIPLimiter,
		txStore.CountUsersByIP,
	)

	ok, err := limiter.Allow(r.Context(), ip)
	if err != nil {
		httpx.InternalError(w, "cannot check ip usage")
		return
	}
	if !ok {
		httpx.TooManyRequests(w)
		return
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		httpx.InternalError(w, "cannot hash password")
		return
	}

	user, err := txStore.CreateUserWithIP(
		r.Context(),
		db.CreateUserWithIPParams{
			Username:     in.Username,
			PasswordHash: hash,
			Ip:           ip,
		},
	)
	if err != nil {
		if IsUniqueConstraint(err) {
			httpx.WriteJSON(w, http.StatusConflict, map[string]any{
				"status":  "error",
				"message": "username already taken",
			})
			return
		}
		httpx.InternalError(w, "cannot create user")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.InternalError(w, "cannot commit transaction")
		return
	}

	httpx.Created(w, map[string]any{
		"id":       user.ID,
		"username": user.Username,
	})
}
