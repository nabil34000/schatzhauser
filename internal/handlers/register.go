package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

type RegisterHandler struct {
	DB                  *sql.DB
	IPRLimiter          *protect.IPRateLimiter
	AccountPerIPLimiter config.AccountPerIPLimiterConfig
	RBodySizeLimiter    config.RBodySizeLimiterSection
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

	slog.Info("register ip", "ip", protect.GetIP(r))

	// Request-level IP rate limiter (optional)
	if h.IPRLimiter != nil && h.IPRLimiter.Enable {
		ip := protect.GetIP(r)
		if ip != "" && !h.IPRLimiter.Allow(ip) {
			tooManyRequests(w)
			return
		}
	}

	// Request Body size limiting
	if h.RBodySizeLimiter.Enable {
		// ---- Content-Length upfront gate
		if r.ContentLength > h.RBodySizeLimiter.MaxRBodyBytes {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}

		// ---- Hard read limit (authoritative)
		if err := protect.LimitRequestBody(w, r, h.RBodySizeLimiter.MaxRBodyBytes); err != nil {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()
	}

	var in RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		badRequest(w, "invalid json")
		return
	}

	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		badRequest(w, "username and password required")
		return
	}

	ip := protect.GetIP(r)

	store := db.NewStore(h.DB)

	// -------------------------------------------------
	// TRANSACTION
	// -------------------------------------------------
	tx, err := h.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		internalError(w, "cannot begin transaction")
		return
	}
	defer tx.Rollback()

	txStore := store.WithTx(tx)

	// 1️⃣ IP limit enforcement (optional, persistent)
	if h.AccountPerIPLimiter.Enable {
		if h.AccountPerIPLimiter.MaxAccounts > 0 && ip != "" {
			count, err := txStore.CountUsersByIP(r.Context(), ip)
			if err != nil {
				internalError(w, "cannot check ip usage")
				return
			}
			if count >= int64(h.AccountPerIPLimiter.MaxAccounts) {
				tooManyRequests(w)
				return
			}
		}
	}

	// 2️⃣ Hash password
	hashed, err := hashPassword(in.Password)
	if err != nil {
		internalError(w, "cannot hash password")
		return
	}

	// 3️⃣ Create user (sqlc)
	user, err := txStore.CreateUserWithIP(
		r.Context(),
		db.CreateUserWithIPParams{
			Username:     in.Username,
			PasswordHash: hashed,
			Ip:           ip,
		},
	)

	if err != nil {
		if isUniqueConstraint(err) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"status":  "error",
				"message": "username already taken",
			})
			return
		}
		internalError(w, "cannot create user")
		return
	}

	// 4️⃣ Commit
	if err := tx.Commit(); err != nil {
		internalError(w, "cannot commit transaction")
		return
	}

	created(w, map[string]any{
		"id":       user.ID,
		"username": user.Username,
	})
}
