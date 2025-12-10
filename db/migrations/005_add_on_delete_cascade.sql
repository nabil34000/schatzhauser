-- 005_add_on_delete_cascade.sql
PRAGMA foreign_keys=off;

BEGIN TRANSACTION;

-- rename old table
ALTER TABLE sessions RENAME TO sessions_old;

-- create new sessions table with ON DELETE CASCADE
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    session_token TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- copy old data
INSERT INTO sessions (id, user_id, session_token, created_at, expires_at)
SELECT id, user_id, session_token, created_at, expires_at
FROM sessions_old;

-- drop old table
DROP TABLE sessions_old;

COMMIT;

PRAGMA foreign_keys=on;
