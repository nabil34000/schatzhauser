-- 004_harden_users_and_roles.sql
-- Enforce valid user roles and prepare schema for patch-style updates

PRAGMA foreign_keys = OFF;

-- 1. Create a replacement table with constraints
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    ip TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user'
        CHECK (role IN ('user', 'admin')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. Copy data safely, coercing invalid roles if any slipped in
INSERT INTO users_new (id, username, password_hash, ip, role, created_at)
SELECT
    id,
    username,
    password_hash,
    ip,
    CASE
        WHEN role IN ('user', 'admin') THEN role
        ELSE 'user'
    END,
    created_at
FROM users;

-- 3. Replace old table
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- 4. Recreate indexes
CREATE INDEX IF NOT EXISTS idx_users_ip ON users(ip);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

PRAGMA foreign_keys = ON;
