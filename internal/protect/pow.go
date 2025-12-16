package protect

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/httpx"
)

/*
────────────────────────────────────────────────────────────
Config
────────────────────────────────────────────────────────────
*/

type PowConfig struct {
	Enable     bool
	Difficulty uint8
	TTL        time.Duration
	SecretKey  []byte
}

/*
────────────────────────────────────────────────────────────
Challenge handler
────────────────────────────────────────────────────────────
*/

type PoWHandler struct {
	Cfg PowConfig
	Key []byte
}

func NewPoWHandler(cfg PowConfig) *PoWHandler {
	return &PoWHandler{
		Cfg: cfg,
		Key: cfg.SecretKey,
	}
}

type challengePayload struct {
	Challenge  string `json:"challenge"`
	Difficulty uint8  `json:"difficulty"`
	TTLSecs    int64  `json:"ttl_secs"`
	Token      string `json:"token"`
}

func (h *PoWHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.Cfg.Enable {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// generate challenge
	ch := make([]byte, 16)
	_, _ = rand.Read(ch)
	chStr := base64.RawStdEncoding.EncodeToString(ch)

	now := time.Now().Unix()
	exp := now + int64(h.Cfg.TTL.Seconds())

	expBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(expBytes, uint64(exp))

	mac := hmac.New(sha256.New, h.Key)
	mac.Write([]byte(chStr))
	mac.Write(expBytes)
	hmacPart := mac.Sum(nil)

	token := base64.RawStdEncoding.EncodeToString(hmacPart) +
		"." +
		base64.RawStdEncoding.EncodeToString(expBytes)

	resp := challengePayload{
		Challenge:  chStr,
		Difficulty: h.Cfg.Difficulty,
		TTLSecs:    exp - now,
		Token:      token,
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

/*
────────────────────────────────────────────────────────────
Guard
────────────────────────────────────────────────────────────
*/

type PoWGuard struct {
	Cfg PowConfig
	Key []byte
}

func NewPoWGuard(cfg PowConfig) *PoWGuard {
	return &PoWGuard{
		Cfg: cfg,
		Key: cfg.SecretKey,
	}
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

/*
────────────────────────────────────────────────────────────
Verification
────────────────────────────────────────────────────────────
*/

// VerifyPoW returns nil if accepted.
func VerifyPoW(cfg PowConfig, key []byte, challenge, nonce, token string) error {
	if !cfg.Enable || cfg.Difficulty == 0 || cfg.TTL <= 0 || len(key) == 0 {
		return nil
	}

	if challenge == "" || nonce == "" || token == "" {
		return errors.New("missing pow fields")
	}

	exp, err := parseToken(key, challenge, token)
	if err != nil {
		return err
	}

	if time.Now().Unix() > exp {
		return errors.New("challenge expired")
	}

	if !checkDifficulty(challenge, nonce, cfg.Difficulty) {
		return errors.New("invalid pow")
	}

	return nil
}

func parseToken(key []byte, challenge, token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errors.New("invalid token format")
	}

	hmacPart, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("bad hmac encoding")
	}

	expRaw, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, errors.New("bad exp encoding")
	}
	exp := int64(binary.BigEndian.Uint64(expRaw))

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(challenge))
	mac.Write(expRaw)
	expected := mac.Sum(nil)

	if !hmac.Equal(expected, hmacPart) {
		return 0, errors.New("bad hmac")
	}

	return exp, nil
}

func checkDifficulty(challenge, nonce string, difficulty uint8) bool {
	chBytes, err := base64.RawStdEncoding.DecodeString(challenge)
	if err != nil {
		return false
	}

	h := sha256.New()
	h.Write(chBytes)
	h.Write([]byte(nonce))
	sum := h.Sum(nil)

	var bits uint8
	for _, b := range sum {
		for i := uint(7); i < 8; i-- {
			if bits == difficulty {
				return true
			}
			if (b>>i)&1 != 0 {
				return false
			}
			bits++
			if bits == difficulty {
				return true
			}
		}
	}
	return bits >= difficulty
}
