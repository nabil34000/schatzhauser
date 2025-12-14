## schatzhauser

This is a Go backend to build protected CRUD apps. No admin GUI/TUIs (they do not scale), no emails, no 3rd party auth providers. KISS, DIY, YAGNI.

The following works already (some polishing will continue):

- [x] register, login/out, profile handlers (routes),

- [x] username/passwd authentication with session cookies,

- [x] maximal request rate per IP (fixed window, in memory),

- [x] maximal account number per IP (persistent in SQLite),

- [x] maximal request body size (register, login),

- [x] proof of work,

- [x] god mode to create, update, list, delete users and change roles,

- [x] tests are independent Go programs, no import "testing",

- [ ] VPS deployment.

Go stdlib for routing, SQLite, sqlc v2 with no SQL in the Go code.

## Architecture

- ./cmd - entrance points to server and god (separate user management cli program)

- ./data/data.db - SQlite db, single file, should be easy to VPS and backup.

- ./db - mostly sqlc generated, except store.go and migrations.go, queries via AI.

- ./internal

  - config/config.go - all the default params, validation, early fatal failures due to missing db path or PoW key.

  - ./handlers - register, login, profile, logout.

    - domain_helpers.go include db, sessions, but low level json and HTTP stuff is in httpx.

  - httpx - x for extras, json and http request helpers used by handlers and protectors.

  - protect - middleware: (i) request guards which are easily called by any handler in any sequence, and (ii) domain-level guards such as max account per ip limiter which is not so easy to abstract and chain, may access db, depend on business logic etc.

  - server/routes.go - this is where all the parameters come from main.go and config/config.go and middleware is assembled, and then what is needed is passed to each handler.

  - config.toml - all the parameters whose default values are in config/config.go via config structs duplicating the main ones (simple Go, no builders and Rob Pike's option structs).

The trickiest part in all this is middleware. The initial versions of this code were very straightforward Go, but the handlers looked ugly and it was hard to reuse anything. It was also very easy to get lost with paranoid existential checks for nil and validations everywhere.

These rules eliminate 80 percent of the mess:

- A guard (protector's common type/interface) is never nil, disabled means inactive, not absent.

- Defaults + validation + fatal errors live in config, no paranoid checks elsewhere.

- Handlers never decide whether a protector is enabled, they only run a fully formed guard or a sequence of them, defined and instantiated for a specific handler inside ./internal/server/routes.go.

- A guard checks if everything is alright and returns true, or writes an HTTP response and returns false. There is no nil, error handling, panic() and other crapola.

- PoW is headers-only, no fallbacks to json body.

## More about Middleware (Guards)

This is the code which runs inside a handler before business logic, but sometimes also gets entangled with it.

Those that run before are in-memory guards. Those which are messier may access db and are excluded into "DIY and put inside a handler".

Most of the guards are **stateless, synchronous, and in-memory request gates**. To chain/execute them in sequence we need to put then under a common type which is done by forcing them to implement the `protect.Guard` interface:

```go
type Guard interface {
    Check(w http.ResponseWriter, r *http.Request) bool
}
```

This lives inside ./internal/protect to break a cycle between ./internal/server/routes.go and ./internal/handlers/.

The rest is just Go code. Inside a handler an active guard will emit an HTTP response and exit earlier via return

```
for _, g := range h.Guards {
    if !g.Check(w, r) {
        return
    }
}
```

See ./internal/handlers/register.go as an example.

You will find the following tested guards inside ./internal/protect:

- ip_rate_guard.go – HTTP request rate per ip limiting.

- body_size_guard.go – request body size cap.

- pow_guard.go – optional Proof-of-Work challenge.

./internal/handlers/register.go also includes a maximal accounts per ip limiter,

```go
limiter := protect.NewAccountPerIPLimiter(
		h.AccountPerIPLimiter,
		txStore.CountUsersByIP,
	)
```

which needs to access db via txStore, which in turn requires further context. This is the guard of the second type mentioned above. It is excluded from middleware in order to avoid unnecessary abstractions. We are going to use the check for the accounts limit per ip only inside the register route (handle) anyway.

This way we cover (more or less) complex Ruby Rack machinery with basic typed Go code. Bear in mind that not everything that can be composed needs to be composed. Abstractions/magic bring hidden cost. YAGNI.

## Setup/Workflow

Clone, cd, and run `make all` which should create two binaries inside ./bin: server and god.

First time (no DB):

```bash
mkdir data && touch data/data.db
sqlc generate
```

After modifying Go code:

```bash
sqlc generate
make
```

After adding a new migration file to db/migrations:

```bash
sqlc generate
make
./bin/server
```

./bin/server is a compiled .cmd/main.go which executes migrations via ./db/migrations.go. You must start/restart the server for migrations to take place.

Any change to DB takes place by creating a new migration file inside ./db/migrations.

The first one, 001_init.sql sets up the schema, so I keep the folder ./db/schema empty not to duplicate stuff. Once a migration takes place by starting ./bin/server, there is no way of modifying these files if you want to do things correctly with a running DB. There is no rolling back, deleting files, this is the SQL world.

Any modification takes place by adding a new migration which can add a variable or delete it via a new transaction which copies and recreates the whole table.

`sqlc generate` is a static tool to generate Go files once a migration file is added, and/or new queries are added inside ./db/queries.sql. It does not execute migrations, it only generates Go inside ./db.

Run cli tool `god` to manage users, SQLite allows that irrespectively whether server is running or not.

```bash
./bin/god user set --username alice --password passw0
./bin/server
time=2025-12-02T22:41:03.389+02:00 level=INFO msg="starting server" debug=true
time=2025-12-02T22:41:03.389+02:00 level=INFO msg="listening on :8080"
^Ctime=2025-12-02T22:41:40.035+02:00 level=INFO msg="shutting down"
```

Adjust config.toml as you wish, but the tests will barf about the right values. The Go code uses defaults where needed if the values are wrong or omitted inside config.toml.

## API

```bash
## API Usage

### Register
curl -i -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"u1","password":"p1"}' \
  http://localhost:8080/api/register

### Login (save cookie)
curl -i -c cookiejar.txt \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"u1","password":"p1"}' \
  http://localhost:8080/api/login

### Profile (authenticated)
curl -i -b cookiejar.txt \
  http://localhost:8080/api/profile

### Logout
curl -i -b cookiejar.txt \
  -X POST \
  http://localhost:8080/api/logout

```

## God Mode

Use god to create/update (set) users, adjust (set) roles, delete users. It is a minimal CLI app which opens the same SQLite database (DB) file directly. It uses the same schema and sqlc queries.

SQLite supports multiple processes safely (file locking handles it), with one caveat. If the server is actively writing at the same moment, SQLite may briefly lock the DB. In that case god will just get a transient error; rerun is fine. So one can run god while the server is up or down.

```bash
# create admins
./bin/god user set --username test_user0 --role admin --password hunter2
./bin/god user set --username salomeja --role admin --password neris

# create regular users
./bin/god user set --username test_user1 --password pass1
./bin/god user set --username test_user2 --password pass2

# get user profile
./bin/god user get salomeja

# list users
./bin/god users list

# create/update user fields including role promotion/demotion and password rotation
./bin/god user set --username test_user0 --role user --password hunter2
./bin/god user set --username test_user1 --role admin --ip 1.2.3.4
./bin/god user set --username test_user0 --password newpass

# delete single user
./bin/god user delete --username test_user1

# bulk delete by prefix
./bin/god users delete --prefix test_

# bulk delete by creation date
./bin/god users delete --created-between 2025-12-02 2025-12-05
```

## Proof of Work (PoW)

This is quite a complication, but it allows to put a secure computational burden on the client side. For a human wasting 1s of the CPU when registering an account is not much. For a bot bombarding the API with millions of requests the computational resources will skyrocket.

This lightweight client-side PoW system protects the /api/register endpoint from automated mass-account creation. When enabled, the server issues a short-lived challenge. The client must find a nonce whose SHA-256 hash meets a difficulty requirement (leading zero bits). Successful solutions permit registration.

PoW is optional, controlled through config.toml:

```toml
[proof_of_work]
enable = true         # enable or disable PoW
difficulty = 22       # number of leading zero bits required
ttl_seconds = 90      # challenge validity time
secret_key = "NN0yfFmJEn0pEWnOYngbd1BfdH9mMUODyw48YRx2FeY="  # secret used to sign challenges (required when enable=true)
```

**Important**: Use your own key. On Linux/macOS generate it with

```bash
openssl rand -base64 32
```

To hide from github commits, use .env. Put .env inside .gitignore, and load .env inside Go with

```go
import "github.com/joho/godotenv"

func main() {
    godotenv.Load()
    // continue…
}
```

If .env is

```
POW_SECRET_KEY=NN0yfFmJEn0pEWnOYngbd1BfdH9mMUODyw48YRx2FeY=
```

then config.toml should use

```
secret_key = "${POW_SECRET_KEY}"
```

I neglect this for now and commit the key on github for demo purposes.

If enable = false, all PoW headers are ignored and registration proceeds normally.

PoW Flow:

Client requests a challenge

Server returns {challenge, difficulty, ttl_secs}

Client brute-forces a nonce

Compute:
hash = sha256(challenge + ":" + nonce)

If leading_zero_bits(hash) >= difficulty, send the solution:

X-PoW-Challenge

X-PoW-Nonce

X-PoW-Hash

Server verifies signature + TTL + difficulty and allows registration.

### 1. Fetch Challenge

```bash
curl -i -X POST \
  http://localhost:8080/api/pow/challenge
```

Successful response (200 OK):

```bash
{
  "challenge": "C7J2xG1fTqs5u1q9gPBFJw==",
  "difficulty": 22,
  "ttl_secs": 90
}
```

If PoW is disabled,

```bash
204 No Content
```

No PoW headers required afterward.

### 2. Solve PoW

On the client:

Increment nonce (0,1,2,...)

Compute SHA-256(challenge + ":" + nonce)

Check number of leading zero bits

When difficulty satisfied, proceed

The test script ./tests/pow_register/main.go does this automatically.

### 3. Submit Registration With Proof

Example (values illustrative only)

```bash
curl -i -X POST \
  -H "Content-Type: application/json" \
  -H "X-PoW-Challenge: C7J2xG1fTqs5u1q9gPBFJw==" \
  -H "X-PoW-Nonce: 2193871" \
  -H "X-PoW-Hash: 000000a1f0b8e9ac..." \
  -d '{"username":"new_user","password":"hunter2"}' \
  http://localhost:8080/api/register

```

Successful response:

```bash
201 Created
{"id":123,"username":"new_user"}
```

Failure (bad PoW):

```bash
400 Bad Request
{"status":"error","message":"invalid pow: difficulty too low"}
```

### 4. Registration When PoW Is Disabled

No PoW headers needed:

```bash
curl -i -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"no_pow","password":"p"}' \
  http://localhost:8080/api/register
```

Why headers instead of bodies in PoW: "Headers keep the PoW metadata separate from the JSON body, which keeps the register payload clean, avoids mixing transport-level proof with application data, lets the server check PoW before reading or parsing the body (protecting body-size limits and parsers), and makes PoW uniformly attachable to any route without changing its JSON schema."

### Proof-of-Work Parameters

- **Difficulty** – number of leading zero bits required in the SHA-256 hash of `challenge || nonce`.

  - Controls computational cost for clients.
  - Example values:
    - Easy: 16 → few milliseconds per solve
    - Moderate: 20 → ~50–100 ms per solve
    - Hard: 24 → ~1 second per solve (CPU-dependent)

- **TTL (Time-To-Live)** – lifetime of a PoW challenge in seconds.

  - Clients must submit a valid nonce before this expires.
  - Prevents reuse of old challenges.
  - Typical values:
    - Short: 15–30s
    - Medium: 60s
    - Long: 120s

Recommended setups:

- Easy (login, also password resets if to be implemented some day):

  - difficulty = 16
  - ttl_seconds = 20

- Moderate (registration, or roughly everywhere):

  - difficulty = 20
  - ttl_seconds = 30

- Hard (abuse mitigation):

  - difficulty = 23
  - ttl_seconds = 20

## Tests

These are independent Go programs. The goal is to test the main feature and make a memo on how something works/breaks. I will not use any autogenerated doc fluff or import "testing".

Start the server, open another terminal and run one of these:

```bash
go run ./tests/register
go run ./tests/login
go run ./tests/profile
go run ./tests/logout
go run ./tests/req_rate_per_ip
go run ./tests/account_rate_per_ip
go run ./tests/req_body_size
go run ./tests/pow_register
```

For example,

```bash
go run ./tests/req_body_size
=== Register small payload ===
[PASS] expected=201 got=201
Response body: {"id":200,"username":"u1_1765413623934059924"}


=== Login small payload ===
[PASS] expected=200 got=200
Response body: {"message":"logged in","status":"ok","username":"u1_1765413623934059924"}


=== Register empty payload ===
[PASS] expected=400 got=400
Response body: {"message":"username and password required","status":"error"}


=== Login empty payload ===
[PASS] expected=400 got=400
Response body: {"message":"username and password required","status":"error"}


=== Register too large payload ===
[PASS] expected=413 got=413
Response body: payload too large


=== Login too large payload ===
[PASS] expected=413 got=413
Response body: payload too large


ALL OK ✅
```

Another one:

```bash
go run ./tests/pow_register
== Running PoW real-register tests ==
[pow_user_1765493640134_0] solving difficulty 22...
✅ pow_user_1765493640134_0 registered OK
[pow_user_1765493640134_1] solving difficulty 22...
✅ pow_user_1765493640134_1 registered OK
[pow_user_1765493640134_2] solving difficulty 22...
✅ pow_user_1765493640134_2 registered OK
[pow_user_1765493640134_3] solving difficulty 22...
✅ pow_user_1765493640134_3 registered OK
[pow_user_1765493640134_4] solving difficulty 22...
✅ pow_user_1765493640134_4 registered OK
🎉 All tests passed!
```

To Do: this is not yet automated into a single test.

## More on Go, SQLite, and sqlc

[How We Went All In on sqlc/pgx for Postgres + Go (2021)](https://brandur.org/sqlc)

[How We Went All In on sqlc... on HN](https://news.ycombinator.com/item?id=28462162)

[Pocketbase – open-source realtime back end in 1 file on HN](https://news.ycombinator.com/item?id=46075320)

[PocketBase: FLOSS/fund sponsorship and UI rewrite #7287](https://github.com/pocketbase/pocketbase/discussions/7287)

[Mat Ryer: How I write HTTP services in Go after 13 years (2024)](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)
