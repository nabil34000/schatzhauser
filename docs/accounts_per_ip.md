## Maximal Account Number per IP

Limiting the request rate per IP slows down a fast attacker, but what if an adversary slowly and periodically creates spam accounts? We can solve this problem by limiting the maximal number of accounts per IP.

One annoying consequence is that Account-per-IP is not "middleware", it belongs to the same db transaction whose life cycle is owned by a handler. The problem is not that a guard touches DB per se. Guards would now need request-scoped dependencies that do not exist at route assembly time.

A few other examples that mix business domain (handler logic) with early guarding is blacklisting IPs or checking if some seat numbers are exceeded in a ticket issuing system.

There is a way to extend these guards with an extra argument. Inside ./internal/protect/guard.go:

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

and then the handler code would be

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

    env := &protect.Env{
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

This is still messy and kind of polluting pure middleware with Env. We do not immediately see which guard demands a DB transaction. There is a way to further split guards into "middleware" and those state accessors or even mutators, policy pipelines and all that. The decision taken here is not to go there at all.

Any guard/middleware is transaction-free, and anything else is just part of the handler's business logic, see ./internal/handlers/register.go for the imposition of the maximal account number per IP.

However complex, this is just code organization, now onto the real stuff ;).

### Modification No. 1

Counting users per ip directly turns out to be not a good idea:

```sql
-- name: CountUsersByIP :one
SELECT COUNT(*) FROM users
WHERE ip = ?;
```

The SQL code above will also count deleted users (unless hard-deleted). A user may have old forgotten accounts, which should not prevent new account creation. The main worry is only consistent spamming to over flood DB.

The solution is to count per time window, not per life-time.

```sql
SELECT COUNT(*) FROM users
WHERE ip = ?
AND created_at >= datetime('now', '-30 days');
```

The actual limit is on the account number per IP per 30 days. max_accounts = 7 inside config.toml implies 7 allowed accounts created per 30 days. An evil user can spam the system with the rate of 6 accounts per 30 days, which is manageable.

### Modification No. 2

There is an edge case with the race condition. Two /register requests from the same IP arrive at nearly the same time.

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
