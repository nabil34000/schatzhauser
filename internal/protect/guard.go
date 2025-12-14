package protect

import "net/http"

/*

Guard is a "request gate", it's an interface implemented by each protection mechanism inside
./internal/protector.

Having this single type we can iterate over or compose multiple protection mechanisms inside handlers,
like the ip rate limiter followed or preceded by request body size limiter, which could also involve PoW.

Those chains/arrays of Guard are constructed in ./internal/server/router.go which passes them on to handlers
with proper cfg parameters received from main.go which called ./internal/config/config.go.

The return value of Check signals whether the request should continue (true) or stop (false).

Writing the response (error, 429, etc.) is the guard’s responsibility if it returns false.

*/

type Guard interface {
	Check(w http.ResponseWriter, r *http.Request) bool
}
