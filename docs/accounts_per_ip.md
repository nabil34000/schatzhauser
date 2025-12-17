## Maximal Account Number per IP

Guards on request rates slow down fast attacks, but what if an adversary slowly and periodically creates spam accounts? Limiting the maximal number of accounts per IP (MANI) aims to solve this, it is included inside

./internal/handlers/register.go

and can be enabled in ./config.toml:

```toml
# Max accounts per IP
[account_per_ip_limiter]
enable = true
max_accounts = 7
```

Code organization and a few edge cases are addressed below.

### MANI is Not Middleware

The problem is that MANI needs access to DB, but the transaction's life cycle is owned by the request handler. To treat MANI as a regular middleware, one would need to extend the Guard interface and drag the whole environment with it:

```go
type Env struct {
    Ctx   context.Context
    DB    *sql.DB
    Tx    *sql.Tx
    Store *db.Store
}

type Guard interface {
    Check(w http.ResponseWriter, r *http.Request, env *Env) bool
}
```

The handler code would then be

```go
func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    tx, err := h.DB.BeginTx(r.Context(), nil)
    if err != nil {
        httpx.InternalError(w, "cannot begin transaction")
        return
    }
    defer tx.Rollback()

    env := &guard.Env{
        Ctx:   r.Context(),
        DB:    h.DB,
        Tx:    tx,
        Store: db.NewStore(h.DB).WithTx(tx),
    }

    for _, g := range h.Guards {
        if !g.Check(w, r, env) {
            return
        }
    }

    // business logic only from here down
    ...
    tx.Commit()
}

```

This is called _policy pipeline_, but it is discarded in this code, let us keep it simple. Any guard/middleware is transaction-free, and anything else will be inside handler's business logic.

### Counting Users (Accounts) per IP

Counting users per ip directly turns out to be not a good idea:

```sql
-- name: CountUsersByIP :one
SELECT COUNT(*) FROM users
WHERE ip = ?;
```

The SQL code above will also count deleted users (unless hard-deleted). A user may have old forgotten accounts, which should not prevent new account creation. Small amounts of spam are tolerable, we worry only about consistent spammers who aim to overflow the system.

The solution is to count per time window, not per life-time.

```sql
SELECT COUNT(*) FROM users
WHERE ip = ?
AND created_at >= datetime('now', '-30 days');
```

The actual limit is on the account number per IP per 30 days. Let max_accounts = 7 inside config.toml, which implies 7 allowed accounts created per 30 days. An evil user can spam the system with the rate of 6 accounts per 30 days, which is manageable.

### Race Condition

There is an edge case to tackle. Let two api/register requests from the same IP arrive at nearly the same time.

Both do:

SELECT COUNT(users WHERE ip = X).

See count = 6 (limit is 7).

Both proceed.

Both INSERT user.

Result: 8 accounts from one IP, limit violated.

Why it happens:

The count check and the insert are two separate statements.

Default isolation allows both transactions to see the same snapshot.

The DB is doing exactly what is asked, but not what is meant.

The solution is to create a dummy query for sqlc:

```sql
-- name: TouchUsersTable :exec
UPDATE users SET ip = ip WHERE ip = ?;
```

leading to this Go code:

```go
if err := txStore.TouchUsersTable(r.Context(), ip); err != nil {
    httpx.InternalError(w, "cannot lock users table")
    return
}
```

It updates zero or more rows, changes nothing, **forces a write lock**.

With that writer lock, when two users register at the same time:

Request A reaches TouchUsersTable -> gets writer lock.

Request B reaches TouchUsersTable -> blocks.

A counts, inserts, commits.

B resumes, recounts, now sees **updated state**, and discards user creation as the limit 7 is already reached.

SQLite internally serializes writers. Readers may still run, writers wait their turn.
