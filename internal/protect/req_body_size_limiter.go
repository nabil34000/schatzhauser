package protect

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	DefaultLoginPayload = 4 << 10  // 4 KB
	MinPayload          = 512      // bytes
	MaxPayload          = 64 << 10 // 64 KB hard ceiling
)

// NormalizePayloadLimit returns a safe payload size.
func NormalizePayloadLimit(v int64) int64 {
	switch {
	case v <= 0:
		return DefaultLoginPayload
	case v < MinPayload:
		return DefaultLoginPayload
	case v > MaxPayload:
		return MaxPayload
	default:
		return v
	}
}

var ErrPayloadTooLarge = errors.New("request payload too large")

// LimitRequestBody limits the maximum number of bytes that can be read
// from r.Body. Call this BEFORE decoding JSON.
//
// Example:
//
//	if err := protect.LimitRequestBody(w, r, 4<<10); err != nil {
//	    http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
//	    return
//	}
func LimitRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	if maxBytes <= 0 {
		return nil // disabled by configuration
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	return nil
}

// IsPayloadTooLarge checks whether an error came from exceeding MaxBytesReader.
func IsPayloadTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// PayloadTooLargeError returns a human-friendly message.
func PayloadTooLargeError(maxBytes int64) error {
	return fmt.Errorf("payload too large (max %d bytes)", maxBytes)
}
