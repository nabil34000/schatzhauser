## schatzhauser

This is a minimal RESTful API server (backend) in Go:

- import "net/http", no chi,

- username/passwd auth with session cookies,

- request rate limiter per IP,

- "middleware" is just Go inside a request handler,

- [Mat Ryer's graceful ctrl+C](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/).

## Motivation

The ideal is [PocketBase](https://pocketbase.io/docs/authentication/), aiming for something even simpler here.

The PocketBase revolution:

```
systemctl enable myapp
systemctl start myapp
journalctl -u myapp -f
```

myapp is a Go binary which updates data.db on the same VPS, plain Linux. No devops yaml "application gateway ingress controllers" from hell. If you are scaling, you are on the wrong side of history.

I use almost no 3rd party, except these:

```bash
go mod init github.com/aabbtree77/schatzhauser
go get github.com/mattn/go-sqlite3
go get golang.org/x/crypto/bcrypt
go get github.com/sqlc-dev/sqlc
go get github.com/BurntSushi/toml
```

## Details

Terminal 1:

```bash
sqlc generate
go build -o schatzhauser .
mkdir -p data
./schatzhauser
time=2025-12-02T22:41:03.389+02:00 level=INFO msg="starting schatzhauser" debug=true
time=2025-12-02T22:41:03.389+02:00 level=INFO msg="listening on :8080"
^Ctime=2025-12-02T22:41:40.035+02:00 level=INFO msg="shutting down"
```

Terminal 2:

```bash
# register
curl -i -X POST -H 'Content-Type: application/json' -d '{"username":"u1","password":"p1"}' http://localhost:8080/register

# login (save cookie)
curl -i -c cookiejar.txt -X POST -H 'Content-Type: application/json' -d '{"username":"u1","password":"p1"}' http://localhost:8080/login

# profile (send cookie)
curl -i -b cookiejar.txt http://localhost:8080/profile

# logout
curl -i -b cookiejar.txt -X POST http://localhost:8080/logout
```

Further tests:

```bash
go run ./tests/profile
Cookies after login: [schatz_sess=267e63d4c1d67ab173ba385cb53929a1db5a80e542f0cae1065209c622c2891e]
PASS: profile with cookie: status=200, body={"status":"ok","user":{"created":{"Time":"2025-12-02T21:31:43Z","Valid":true},"id":19,"username":"profile_test_1764711103328518799"}}

PASS: profile without cookie: got 401 as expected
=== SUMMARY: 1/1 passed ===
```

```
go run ./tests/register
go run ./tests/login
go run ./tests/profile
go run ./tests/logout
go run ./tests/ip_rate_limit
```

Comment out the IP rate limiters when testing anything but `ip_rate_limit`. In the latter, make sure config.toml [ip_rate_limiter.login] section params match the ones at the start of func main() inside tests/ip_rate_limit/main.go.

The IP rate limiter is a simple-looking fixed window counter, but it is already the second version as the first one leaked memory. Tricky...

Ask AI to write an industrial grade IP rate limiter, but bear in mind the codes which are hard to understand will be even harder to debug, so I follow KISS here.

## Coming Soon

- [ ] Maximal number of registered accounts per IP.

- [ ] Proof of work to slow down spam bots.

- [ ] HTTP request body size limiter.

- [ ] Session expiry.

- [ ] IP bans.
