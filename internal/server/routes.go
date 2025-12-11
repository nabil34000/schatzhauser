package server

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/handlers"
	"github.com/aabbtree77/schatzhauser/internal/protect"
)

// Note the capital "R"... as this is used inside ./cmd/server
func RegisterRoutes(mux *http.ServeMux, db *sql.DB,
	cfg *config.Config) {

	mux.Handle("/api/register", &handlers.RegisterHandler{
		DB:                  db,
		IPRLimiter:          protect.NewIPRateLimiter(tomlSect2IPRConfig(cfg.IPRateLimiter.Register)),
		AccountPerIPLimiter: cfg.AccountPerIPLimiter,
		RBodySizeLimiter:    validateParamsRBodySizeLimiter(cfg.RBodySizeLimiter.Register),
	})

	mux.Handle("/api/login", &handlers.LoginHandler{
		DB:               db,
		IPRLimiter:       protect.NewIPRateLimiter(tomlSect2IPRConfig(cfg.IPRateLimiter.Login)),
		RBodySizeLimiter: validateParamsRBodySizeLimiter(cfg.RBodySizeLimiter.Login),
	})

	mux.Handle("/api/logout", &handlers.LogoutHandler{
		DB:         db,
		IPRLimiter: protect.NewIPRateLimiter(tomlSect2IPRConfig(cfg.IPRateLimiter.Logout)),
	})

	mux.Handle("/api/profile", &handlers.ProfileHandler{
		DB:         db,
		IPRLimiter: protect.NewIPRateLimiter(tomlSect2IPRConfig(cfg.IPRateLimiter.Profile)),
	})
}

/* Don't miss to pass sec.Enable here as the default will always be false
and the IP rate limiter won't run inside the handler.
There is no way to specify default Enable inside protect/ip_rate_limiter.go.
*/

func tomlSect2IPRConfig(sec config.IPRateLimiterSection) protect.IPRateLimiterConfig {
	IPRConfig := protect.IPRateLimiterConfig{}
	IPRConfig.Enable = sec.Enable
	IPRConfig.MaxRequests = sec.MaxRequests
	IPRConfig.Window = time.Duration(sec.WindowMS) * time.Millisecond
	return IPRConfig
}

/*
This is nicer to do directly inside a handler,
but to save a bit of computation, prevalidate stuff here.
*/

func validateParamsRBodySizeLimiter(sec config.RBodySizeLimiterSection) config.RBodySizeLimiterSection {
	if !sec.Enable {
		sec.MaxRBodyBytes = 0
	}
	sec.MaxRBodyBytes = protect.NormalizePayloadLimit(sec.MaxRBodyBytes)
	return sec
}

/*
TDD: check and set every default value if a param in config.toml is missing.
*/
