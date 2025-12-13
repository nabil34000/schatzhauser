package protect

import (
	"errors"
	"fmt"
	"net/http"
)

var ErrPayloadTooLarge = errors.New("request payload too large")

type RBodySizeLimiter struct {
	Enable   bool
	MaxBytes int64
}

func NewRBodySizeLimiter(enable bool, maxBytes int64) *RBodySizeLimiter {
	return &RBodySizeLimiter{
		Enable:   enable,
		MaxBytes: maxBytes,
	}
}

func (l *RBodySizeLimiter) Apply(w http.ResponseWriter, r *http.Request) error {
	if !l.Enable {
		return nil
	}
	if l.MaxBytes <= 0 {
		return nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, l.MaxBytes)
	return nil
}

func IsPayloadTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

func PayloadTooLargeError(maxBytes int64) error {
	return fmt.Errorf("payload too large (max %d bytes)", maxBytes)
}
