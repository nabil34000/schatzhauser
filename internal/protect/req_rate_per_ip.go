package protect

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type IPRateLimiter struct {
	mu              sync.Mutex
	entries         map[string]int
	MaxRequests     int
	Window          time.Duration
	currWindowStart time.Time
	Enable          bool
}

type IPRateLimiterConfig struct {
	Enable      bool
	MaxRequests int
	Window      time.Duration
}

func NewIPRateLimiter(cfg IPRateLimiterConfig) *IPRateLimiter {
	return &IPRateLimiter{
		Enable:          cfg.Enable,
		MaxRequests:     cfg.MaxRequests,
		Window:          cfg.Window,
		entries:         make(map[string]int),
		currWindowStart: time.Now(),
	}
}

// Allow implements a fixed-window rate limiter with **zero memory leaks**.
// Every window reset discards all stored IP counts.
func (rl *IPRateLimiter) Allow(ip string) bool {
	if !rl.Enable {
		return true
	}
	if rl.MaxRequests <= 0 || rl.Window <= 0 || ip == "" {
		return true
	}

	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.currWindowStart) >= rl.Window {
		rl.entries = make(map[string]int)
		rl.currWindowStart = now
	}

	if rl.entries[ip] >= rl.MaxRequests {
		return false
	}

	rl.entries[ip]++
	return true
}

// For debugging
func (rl *IPRateLimiter) Inspect(key string) (count int, start time.Time, found bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	count, ok := rl.entries[key]
	return count, rl.currWindowStart, ok
}

/*
func GetIP(r *http.Request) string {
	hostPort := r.RemoteAddr
	if hostPort == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return strings.Trim(hostPort, "[]")
	}
	return host
}
*/

func GetIP(r *http.Request) string {
	// ✅ DEV / TEST OVERRIDE
	if ip := r.Header.Get("X-Test-IP"); ip != "" {
		return ip
	}

	// PROD: behind trusted proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// optionally gate this by env
		//return parseFirstIP(xff)
		return xff
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return ""
}
