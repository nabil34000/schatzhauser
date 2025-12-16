package protect

import (
	"net/http"

	"github.com/aabbtree77/schatzhauser/internal/httpx"
)

//
// ──────────────────────────────────────────────
// Guard
// ──────────────────────────────────────────────
//

// BodySizeGuard limits the maximum request body size.
type BodySizeGuard struct {
	enable  bool
	maxSize int64
}

func NewBodySizeGuard(enable bool, maxBytes int64) *BodySizeGuard {
	return &BodySizeGuard{
		enable:  enable,
		maxSize: maxBytes,
	}
}

func (g *BodySizeGuard) Name() string {
	return "body-size-limit"
}

func (g *BodySizeGuard) Check(w http.ResponseWriter, r *http.Request) bool {
	if !g.enable || g.maxSize <= 0 {
		return true
	}

	// Fast path: Content-Length known and already too large
	if r.ContentLength > g.maxSize {
		httpx.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"status":  "error",
			"message": "request body too large",
		})
		return false
	}

	// Hard protection: cap reader
	r.Body = http.MaxBytesReader(w, r.Body, g.maxSize)
	return true
}
