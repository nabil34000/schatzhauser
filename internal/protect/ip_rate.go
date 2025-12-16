package protect

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/httpx"
)

//
// ──────────────────────────────────────────────
// Config
// ──────────────────────────────────────────────
//

type IPRateLimiterConfig struct {
	Enable      bool
	MaxRequests int
	Window      time.Duration
}

//
// ──────────────────────────────────────────────
// Guard
// ──────────────────────────────────────────────
//

// IPRateGuard is a fixed-window per-IP rate limiting guard.
type IPRateGuard struct {
	mu              sync.Mutex
	entries         map[string]int
	maxRequests     int
	window          time.Duration
	currWindowStart time.Time
	enable          bool
}

func NewIPRateGuard(cfg IPRateLimiterConfig) *IPRateGuard {
	return &IPRateGuard{
		enable:          cfg.Enable,
		maxRequests:     cfg.MaxRequests,
		window:          cfg.Window,
		entries:         make(map[string]int),
		currWindowStart: time.Now(),
	}
}

func (g *IPRateGuard) Name() string {
	return "ip-rate-limit"
}

func (g *IPRateGuard) Check(w http.ResponseWriter, r *http.Request) bool {
	if !g.enable {
		return true
	}

	ip := GetIP(r)
	if ip == "" || g.maxRequests <= 0 || g.window <= 0 {
		return true
	}

	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	// Window rollover
	if now.Sub(g.currWindowStart) >= g.window {
		g.entries = make(map[string]int)
		g.currWindowStart = now
	}

	if g.entries[ip] >= g.maxRequests {
		httpx.TooManyRequests(w)
		return false
	}

	g.entries[ip]++
	return true
}

//
// ──────────────────────────────────────────────
// IP extraction
// ──────────────────────────────────────────────
//

func GetIP(r *http.Request) string {
	// DEV / TEST override
	if ip := r.Header.Get("X-Test-IP"); ip != "" {
		return ip
	}

	// Behind proxy (assumed trusted at infra level)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return ""
}
