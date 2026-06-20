-- +goose Up
CREATE TABLE user_sessions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    expires_at DATETIME(3) NOT NULL,
    last_seen_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    UNIQUE KEY uk_user_sessions_token_hash (token_hash),
    KEY idx_user_sessions_user_status (user_id, status),
    KEY idx_user_sessions_expires_at (expires_at),
    CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE email_verification_codes (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NOT NULL,
    consumed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_email_codes_lookup (email, purpose, status, created_at),
    CONSTRAINT chk_email_codes_attempt_nonnegative CHECK (attempt_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS email_verification_codes;
DROP TABLE IF EXISTS user_sessions;
