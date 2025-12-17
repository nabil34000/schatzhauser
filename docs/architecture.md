## Architecture

- ./cmd - entrance points to `server` and `god` (independent user management cli).

- ./data/data.db - SQlite DB. Populate it via REST API or ./bin/god. Tests will directly pollute it, use ./bin/god to clean up.

- ./db - mostly sqlc generated, except store.go and migrations.go.

- ./internal

  - config/config.go - default params, validation, fatal failures.

  - ./handlers

    - register, login, profile, logout.

    - domain_helpers.go related to db, sessions, not json/http stuff.

  - httpx - x for extras, json/http helpers used by both: handlers and guards.

  - guards - middleware, mostly to fight bots. Guards do not touch DB.

  - server/routes.go - this is where all the middleware and handlers are assembled.

  - config.toml - user defined parameters with defaults inside config.go.

These rules eliminate 80 percent of the mess:

- A guard is never nil, disabled means inactive, not absent.

- Defaults + validation + fatal errors live in config, no paranoid checks elsewhere.

- Handlers never mess with a guard, they only run a fully formed guard or a sequence of them, defined and instantiated for a specific handler inside ./internal/server/routes.go.

- A guard checks if everything is alright and returns true, or writes an HTTP response and returns false. There is no nil, error handling, panic, and exit maze.

## More about Middleware (Guards)

This is ./internal/guards code which runs inside a handler before business logic. It is **synchronous, and in-memory**. To chain/execute the guards in sequence we put them under a common type `guards.Guard`:

```go
type Guard interface {
    Check(w http.ResponseWriter, r *http.Request) bool
}
```

The package guards breaks a cycle between ./internal/server/routes.go and ./internal/handlers.

Inside a handler an active guard will emit an HTTP response and return false. The handler exits before business logic via return:

```go
for _, g := range h.Guards {
    if !g.Check(w, r) {
        return
    }
}
```

See ./internal/handlers/register.go as an example.

You will find the following tested guards inside ./internal/guards:

- ip_rate.go – HTTP request rate per ip limiting.

- body_size.go – request body size cap.

- pow.go – optional [Proof-of-Work](docs/proof_of_work.md) challenge.

./internal/handlers/register.go also includes the Maximal Accounts per IP limiter,

```go
limiter := guards.NewAccountPerIPLimiter(
		h.AccountPerIPLimiter,
		txStore.CountUsersByIP,
	)
```

It needs to access db via txStore, which runs a transaction controlled by the api/register handler. Treat those stateful guards as a handler's business logic, more details in [Accounts per IP](docs/accounts_per_ip.md) and ./internal/handlers/register.go.

This way we cover (more or less) complex Ruby Rack machinery with basic typed Go code.
