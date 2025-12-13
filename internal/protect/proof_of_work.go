package protect

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type PowConfig struct {
	Enable     bool
	Difficulty uint8
	TTL        time.Duration
	SecretKey  []byte
}

type PoWHandler struct {
	Cfg        PowConfig
	signingKey []byte
}

func NewPoWHandler(cfg PowConfig) http.Handler {
	return &PoWHandler{
		Cfg:        cfg,
		signingKey: cfg.SecretKey,
	}
}

/*
This avoids concrete type access in routes.

Instead of:

	powKey := powHandler.(*protect.PoWHandler).Key

Use:

	powKey := powHandler.(protect.PoWKeyProvider).GetPoWSigningKey()
*/
type PoWKeyProvider interface {
	GetPoWSigningKey() []byte
}

func (h *PoWHandler) GetPoWSigningKey() []byte {
	return h.signingKey
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
		// PoW disabled → return HTTP 204 No Content
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// === Challenge creation ===
	ch := make([]byte, 16)
	rand.Read(ch)
	chStr := base64.RawStdEncoding.EncodeToString(ch)

	now := time.Now().Unix()
	exp := now + int64(h.Cfg.TTL.Seconds())

	// Compute HMAC(secret, challenge || expBytes)
	expBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(expBytes, uint64(exp))

	mac := hmac.New(sha256.New, h.signingKey)
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
	writeJSON(w, http.StatusOK, resp)
}

// VerifyPoW returns nil if accepted.
func VerifyPoW(cfg PowConfig, key []byte, challenge, nonce, token string) error {

	// PoW disabled or misconfigured → no-op
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

	// Recompute expected HMAC
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

	var bitsChecked uint8
	for _, b := range sum {
		for i := uint(7); i < 8; i-- {
			if bitsChecked == difficulty {
				return true
			}
			if (b>>i)&1 != 0 {
				return false
			}
			bitsChecked++
			if bitsChecked == difficulty {
				return true
			}
		}
	}
	return bitsChecked >= difficulty
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
