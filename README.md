## schatzhauser

This is a RESTful API server (backend) in Go. The ideal is [PocketBase](https://pocketbase.io/docs/authentication/), aiming for something smaller here: no GUI/TUIs (they do not scale), no emails, no multiple auth providers, focus on reliability and dev-centric usability.

- [x] username/passwd auth with session cookies,

- [x] maximal request rate per IP (fixed window, in memory),

- [x] maximal account number per IP (persistent in SQLite),

- [x] god mode to create, update, list, delete users and change roles,

- [x] tests as examples (independent Go programs, no import "testing", no faking).

The code uses only Go stdlib for routing, SQLite, sqlc v2 with no SQL in the Go code. Middleware is just Go inside a request handler.

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

The first one, 001_init.sql sets up the schema, so I keep the folder ./db/schema empty not to duplicate stuff. Once a migration takes place by starting ./bin/server, there is no way of modifying these files if you want to do things correctly with a running DB. There is no way rolling back, deleting files, this is the SQL world.

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

## Middleware

Just write Go code inside handlers, do repeat yourself. No composition games, no shenanigans.
The included examples are

./internal/protect/ip_rate_limiter.go

and the other is directly inside the route

./internal/handlers/register.go

as it limits the registered accounts per ip.

TDD: This needs to be better/properly isolated.

## Tests

These are independent Go programs, somewhat messy. The goal is to test the main feature and make a memo on how something works. I will not use any autogenerated doc fluff and import "testing", only real usable programs showing how things work/break.

TBC...

## References

[How We Went All In on sqlc/pgx for Postgres + Go (2021)](https://brandur.org/sqlc)

[How We Went All In on sqlc... on HN](https://news.ycombinator.com/item?id=28462162)

[Pocketbase – open-source realtime back end in 1 file on HN](https://news.ycombinator.com/item?id=46075320)

[PocketBase: FLOSS/fund sponsorship and UI rewrite #7287](https://github.com/pocketbase/pocketbase/discussions/7287)

[Mat Ryer: How I write HTTP services in Go after 13 years (2024)](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)
