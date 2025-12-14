package protect

import (
	"net/http"

	"github.com/aabbtree77/schatzhauser/internal/httpx"
)

type PoWGuard struct {
	Cfg PowConfig
	Key []byte
}

func NewPoWGuard(cfg PowConfig, key []byte) *PoWGuard {
	return &PoWGuard{
		Cfg: cfg,
		Key: key,
	}
}

func (g *PoWGuard) Name() string {
	return "proof-of-work"
}

func (g *PoWGuard) Check(w http.ResponseWriter, r *http.Request) bool {
	if !g.Cfg.Enable {
		return true
	}

	challenge := r.Header.Get("X-PoW-Challenge")
	nonce := r.Header.Get("X-PoW-Nonce")
	token := r.Header.Get("X-PoW-Token")

	if challenge == "" || nonce == "" || token == "" {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"status":  "error",
			"message": "proof of work required",
		})
		return false
	}

	if err := VerifyPoW(g.Cfg, g.Key, challenge, nonce, token); err != nil {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"status":  "error",
			"message": "invalid proof of work",
		})
		return false
	}

	return true
}
