package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/aabbtree77/schatzhauser/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "schatz_sess"
	SessionDuration   = 30 * 24 * time.Hour
	TokenByteLen      = 32
)

//
// ──────────────────────────────────────────────
// Password helpers
// ──────────────────────────────────────────────
//

func HashPassword(pwd string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(h), err
}

func ComparePassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}

//
// ──────────────────────────────────────────────
// Session token helpers
// ──────────────────────────────────────────────
//

func GenerateSessionToken() (string, error) {
	b := make([]byte, TokenByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

//
// ──────────────────────────────────────────────
// Cookie helpers
// ──────────────────────────────────────────────
//

func SetSessionCookie(
	w http.ResponseWriter,
	r *http.Request,
	token string,
	expires time.Time,
) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	}

	if r.TLS != nil {
		c.Secure = true
	}

	http.SetCookie(w, c)
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}

	if r.TLS != nil {
		c.Secure = true
	}

	http.SetCookie(w, c)
}

//
// ──────────────────────────────────────────────
// Session loading helper
// ──────────────────────────────────────────────
//

func GetSessionFromRequest(
	ctx context.Context,
	sqlDB *sql.DB,
	r *http.Request,
) (*db.Session, error) {

	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, err
	}

	store := db.NewStore(sqlDB)

	sess, err := store.GetSessionByToken(ctx, c.Value)
	if err != nil {
		return nil, err
	}

	if time.Now().After(sess.ExpiresAt) {
		_ = store.DeleteSessionByToken(ctx, c.Value)
		return nil, sql.ErrNoRows
	}

	return &sess, nil
}

//
// ──────────────────────────────────────────────
// DB helpers
// ──────────────────────────────────────────────
//

func IsUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
