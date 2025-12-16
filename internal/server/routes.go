package server

import (
	"database/sql"
	"net/http"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/handlers"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

// RegisterRoutes binds all HTTP routes to the stdlib mux.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, cfg *config.Config) {

	// ────────────────────────────────────────
	// Proof of Work (shared)
	// ────────────────────────────────────────

	powCfg := protect.PowConfig{
		Enable:     cfg.ProofOfWork.Enable,
		Difficulty: cfg.ProofOfWork.Difficulty,
		TTL:        cfg.ProofOfWork.TTL(),
		SecretKey:  cfg.ProofOfWork.DecodedSecretKey,
	}

	powHandler := protect.NewPoWHandler(powCfg)
	mux.Handle("/api/pow/challenge", powHandler)

	// ────────────────────────────────────────
	// Register
	// ────────────────────────────────────────

	registerIPR := protect.NewIPRateGuard(protect.IPRateLimiterConfig{
		Enable:      cfg.IPRateLimiter.Register.Enable,
		MaxRequests: cfg.IPRateLimiter.Register.MaxRequests,
		Window:      cfg.IPRateLimiter.Register.Window(),
	})

	registerBody := protect.NewBodySizeGuard(
		cfg.RBodySizeLimiter.Register.Enable,
		cfg.RBodySizeLimiter.Register.MaxRBodyBytes,
	)

	registerGuards := []protect.Guard{
		registerIPR,
		registerBody,
		protect.NewPoWGuard(powCfg),
	}

	mux.Handle("/api/register", &handlers.RegisterHandler{
		DB:     db,
		Guards: registerGuards,
		// AccountPerIPLimiter can remain here if needed later
		AccountPerIPLimiter: cfg.AccountPerIPLimiter,
	})

	// ────────────────────────────────────────
	// Login
	// ────────────────────────────────────────

	loginIPR := protect.NewIPRateGuard(protect.IPRateLimiterConfig{
		Enable:      cfg.IPRateLimiter.Login.Enable,
		MaxRequests: cfg.IPRateLimiter.Login.MaxRequests,
		Window:      cfg.IPRateLimiter.Login.Window(),
	})

	loginBody := protect.NewBodySizeGuard(
		cfg.RBodySizeLimiter.Login.Enable,
		cfg.RBodySizeLimiter.Login.MaxRBodyBytes,
	)

	loginGuards := []protect.Guard{
		loginIPR,
		loginBody,
	}

	mux.Handle("/api/login", &handlers.LoginHandler{
		DB:     db,
		Guards: loginGuards,
	})

	// ────────────────────────────────────────
	// Logout
	// ────────────────────────────────────────

	logoutIPR := protect.NewIPRateGuard(protect.IPRateLimiterConfig{
		Enable:      cfg.IPRateLimiter.Logout.Enable,
		MaxRequests: cfg.IPRateLimiter.Logout.MaxRequests,
		Window:      cfg.IPRateLimiter.Logout.Window(),
	})

	logoutGuards := []protect.Guard{
		logoutIPR,
	}

	mux.Handle("/api/logout", &handlers.LogoutHandler{
		DB:     db,
		Guards: logoutGuards,
	})

	// ────────────────────────────────────────
	// Profile
	// ────────────────────────────────────────

	profileIPR := protect.NewIPRateGuard(protect.IPRateLimiterConfig{
		Enable:      cfg.IPRateLimiter.Profile.Enable,
		MaxRequests: cfg.IPRateLimiter.Profile.MaxRequests,
		Window:      cfg.IPRateLimiter.Profile.Window(),
	})

	profileGuards := []protect.Guard{
		profileIPR,
	}

	mux.Handle("/api/profile", &handlers.ProfileHandler{
		DB:     db,
		Guards: profileGuards,
	})
}
