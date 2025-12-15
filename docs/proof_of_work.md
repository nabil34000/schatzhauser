## Proof of Work

### 0. Overview

The PoW system protects /api/register from automated mass-account creation. When enabled, the server issues a short-lived challenge. The client must find a nonce whose SHA-256 hash meets a difficulty requirement (leading zero bits). Successful solutions permit registration.

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

### 5. Proof-of-Work Parameters

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
