package protect

import (
	"context"

	"github.com/aabbtree77/schatzhauser/internal/config"
)

type AccountPerIPLimiter struct {
	Enable      bool
	MaxAccounts int
	CountFn     func(ctx context.Context, ip string) (int64, error)
}

func NewAccountPerIPLimiter(cfg config.AccountPerIPLimiterConfig, countFn func(context.Context, string) (int64, error)) *AccountPerIPLimiter {
	return &AccountPerIPLimiter{
		Enable:      cfg.Enable,
		MaxAccounts: cfg.MaxAccounts,
		CountFn:     countFn,
	}
}

func (l *AccountPerIPLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	if !l.Enable {
		return true, nil
	}
	if l.MaxAccounts <= 0 || ip == "" {
		return true, nil
	}

	count, err := l.CountFn(ctx, ip)
	if err != nil {
		return false, err
	}
	return count < int64(l.MaxAccounts), nil
}
