-- name: CreateUser :one
INSERT INTO users (username, password_hash, created_at)
VALUES (?, ?, datetime('now'))
RETURNING id, username, password_hash, created_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, created_at
FROM users
WHERE username = ?
LIMIT 1;

-- name: GetUserByID :one
SELECT id, username, password_hash, created_at
FROM users
WHERE id = ?
LIMIT 1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, session_token, created_at, expires_at)
VALUES (?, ?, datetime('now'), ?)
RETURNING id, user_id, session_token, created_at, expires_at;

-- name: GetSessionByToken :one
SELECT id, user_id, session_token, created_at, expires_at
FROM sessions
WHERE session_token = ?
LIMIT 1;

-- name: DeleteSessionByToken :exec
DELETE FROM sessions WHERE session_token = ?;

-- name: CountUsersByIP :one
SELECT COUNT(*) FROM users
WHERE ip = ?;

-- name: CreateUserWithIP :one
INSERT INTO users (username, password_hash, ip, created_at)
VALUES (?, ?, ?, datetime('now'))
RETURNING id, username, password_hash, created_at;

-- name: CreateUserWithRole :one
INSERT INTO users (username, password_hash, ip, role, created_at)
VALUES (?, ?, ?, ?, datetime('now'))
RETURNING id, username, password_hash, ip, role, created_at;


-- name: GetUserFullByUsername :one
SELECT id, username, password_hash, ip, role, created_at
FROM users
WHERE username = ?
LIMIT 1;

-- name: UpdateUserPatch :one
-- Update any subset of password_hash/ip/role. Supply NULL to leave value unchanged.
UPDATE users SET
    password_hash = COALESCE(?, password_hash),
    ip            = COALESCE(?, ip),
    role          = COALESCE(?, role)
WHERE username = ?
RETURNING id, username, password_hash, ip, role, created_at;

-- name: DeleteUserByUsername :exec
DELETE FROM users
WHERE username = ?;

-- name: DeleteUsersByPrefix :exec
-- supply prefix like 'test_' (we append '%' in code, but you can also pass prefix||'%')
DELETE FROM users
WHERE username LIKE ?;

-- name: DeleteUsersCreatedBetween :exec
-- Delete users created_at >= start AND created_at < end
-- param start: timestamp
-- param end: timestamp
DELETE FROM users
WHERE created_at >= :start AND created_at < :end;


-- name: ListUsers :many
SELECT id, username, password_hash, ip, role, created_at
FROM users
ORDER BY created_at DESC;


