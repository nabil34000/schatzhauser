package handlers

import (
	"database/sql"
	"encoding/json"
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
	RBodySizeLimiter    *protect.RBodySizeLimiter

	PoWCfg protect.PowConfig
	PoWKey []byte // comes from routes.go via NewPoWHandler
}

// powJSON expects the client to send the issued token (hmac.exp) when doing body-fallback.
type powJSON struct {
	Challenge string `json:"challenge"`
	Nonce     string `json:"nonce"`
	Token     string `json:"token"`
}

type RegisterInput struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Pow      *powJSON `json:"pow,omitempty"`
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//slog.Info("register ip", "ip", protect.GetIP(r))

	if h.IPRLimiter != nil {
		ip := protect.GetIP(r)
		if !h.IPRLimiter.Allow(ip) {
			tooManyRequests(w)
			return
		}
	}

	// If PoW is enabled we prefer header-based verification (fast reject)
	if h.PoWCfg.Enable {
		// Try header-based PoW first (fast path). We only need headers to verify.
		challenge := r.Header.Get("X-PoW-Challenge")
		nonce := r.Header.Get("X-PoW-Nonce")
		token := r.Header.Get("X-PoW-Token")

		if challenge != "" || nonce != "" || token != "" {
			// If any header present, require all and verify now.
			if err := protect.VerifyPoW(h.PoWCfg, h.PoWKey, challenge, nonce, token); err != nil {
				badRequest(w, "invalid pow: "+err.Error())
				return
			}
			// header-based PoW succeeded; proceed to body parsing below
		}
		// If headers were absent, we will fall back to body PoW after size checks and decode.
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

	// Now decode the registration payload
	var in RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		badRequest(w, "invalid json")
		return
	}

	// If PoW enabled and header-based verification wasn't performed earlier,
	// expect PoW inside the body (back-compat).
	if h.PoWCfg.Enable {
		// If header path was used and verified, we don't require body pow.
		// If headers were absent earlier, require pow in body.
		if r.Header.Get("X-PoW-Challenge") == "" && r.Header.Get("X-PoW-Nonce") == "" && r.Header.Get("X-PoW-Token") == "" {
			if in.Pow == nil {
				badRequest(w, "pow required")
				return
			}
			if err := protect.VerifyPoW(h.PoWCfg, h.PoWKey, in.Pow.Challenge, in.Pow.Nonce, in.Pow.Token); err != nil {
				badRequest(w, "invalid pow: "+err.Error())
				return
			}
		}
	}

	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		badRequest(w, "username and password required")
		return
	}

	ip := protect.GetIP(r)

	store := db.NewStore(h.DB)

	// -------------------------------------------------
	// TRANSACTION starts AFTER PoW is validated
	// -------------------------------------------------
	tx, err := h.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		internalError(w, "cannot begin transaction")
		return
	}
	defer tx.Rollback()

	txStore := store.WithTx(tx)

	// 1️⃣ Max accounts per IP limit enforcement (optional, persistent)
	// Build a per-request limiter so the count runs inside the current tx
	limiter := protect.NewAccountPerIPLimiter(h.AccountPerIPLimiter, txStore.CountUsersByIP)

	ok, err := limiter.Allow(r.Context(), ip)
	if err != nil {
		internalError(w, "cannot check ip usage")
		return
	}
	if !ok {
		tooManyRequests(w)
		return
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
