-- +goose Up
CREATE TABLE users (
    id VARCHAR(32) PRIMARY KEY,
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    primary_email VARCHAR(320) NULL,
    email_verified_at DATETIME(3) NULL,
    terms_accepted_at DATETIME(3) NULL,
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    UNIQUE KEY uk_users_primary_email (primary_email),
    KEY idx_users_status (status),
    KEY idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE account_identities (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(191) NOT NULL,
    email VARCHAR(320) NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
    linked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_account_identities_provider_subject (provider, provider_subject),
    UNIQUE KEY uk_account_identities_user_provider (user_id, provider),
    KEY idx_account_identities_user_id (user_id),
    CONSTRAINT fk_account_identities_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspaces (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(160) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    owner_user_id VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    trial_granted_at DATETIME(3) NULL,
    created_by_user_id VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    UNIQUE KEY uk_workspaces_slug (slug),
    KEY idx_workspaces_owner_user_id (owner_user_id),
    KEY idx_workspaces_created_by_user_id (created_by_user_id),
    KEY idx_workspaces_deleted_at (deleted_at),
    CONSTRAINT fk_workspaces_owner_user FOREIGN KEY (owner_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspaces_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_runtime_accesses (
    workspace_id VARCHAR(32) PRIMARY KEY,
    scope_type VARCHAR(32) NOT NULL,
    scope_code VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    activated_at DATETIME(3) NULL,
    activated_by VARCHAR(128) NOT NULL DEFAULT '',
    disabled_at DATETIME(3) NULL,
    disabled_by VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    KEY idx_workspace_runtime_access_scope (scope_type, scope_code),
    KEY idx_workspace_runtime_access_status (status),
    CONSTRAINT fk_workspace_runtime_accesses_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_members (
    workspace_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    joined_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (workspace_id, user_id),
    KEY idx_workspace_members_user_id (user_id),
    CONSTRAINT fk_workspace_members_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_invitations (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    email VARCHAR(320) NOT NULL,
    role VARCHAR(32) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    invited_by_user_id VARCHAR(32) NOT NULL,
    accepted_by_user_id VARCHAR(32) NULL,
    expires_at DATETIME(3) NOT NULL,
    accepted_at DATETIME(3) NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_workspace_invitations_token_hash (token_hash),
    KEY idx_workspace_invitations_workspace_email (workspace_id, email),
    KEY idx_workspace_invitations_status (status),
    CONSTRAINT fk_workspace_invitations_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_invitations_invited_by FOREIGN KEY (invited_by_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspace_invitations_accepted_by FOREIGN KEY (accepted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE api_keys (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    name VARCHAR(160) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    secret_last4 VARCHAR(8) NOT NULL,
    key_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_by_user_id VARCHAR(32) NOT NULL,
    expires_at DATETIME(3) NULL,
    daily_limit_micro_cny BIGINT NULL,
    monthly_limit_micro_cny BIGINT NULL,
    last_used_at DATETIME(3) NULL,
    total_spend_micro_cny BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    UNIQUE KEY uk_api_keys_key_hash (key_hash),
    KEY idx_api_keys_workspace_id (workspace_id),
    KEY idx_api_keys_created_by_user_id (created_by_user_id),
    KEY idx_api_keys_status (status),
    CONSTRAINT fk_api_keys_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_api_keys_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_model_permissions (
    workspace_id VARCHAR(32) NOT NULL,
    model_id VARCHAR(191) NOT NULL,
    source VARCHAR(32) NOT NULL,
    granted_by_user_id VARCHAR(32) NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (workspace_id, model_id),
    KEY idx_workspace_model_permissions_model_id (model_id),
    CONSTRAINT fk_workspace_model_permissions_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_model_permissions_granted_by FOREIGN KEY (granted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE api_key_model_whitelists (
    api_key_id VARCHAR(32) NOT NULL,
    model_id VARCHAR(191) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (api_key_id, model_id),
    KEY idx_api_key_model_whitelists_model_id (model_id),
    CONSTRAINT fk_api_key_model_whitelists_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_balances (
    workspace_id VARCHAR(32) PRIMARY KEY,
    available_micro_cny BIGINT NOT NULL DEFAULT 0,
    frozen_micro_cny BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at DATETIME(3) NOT NULL,
    CONSTRAINT chk_workspace_balances_available_nonnegative CHECK (available_micro_cny >= 0),
    CONSTRAINT chk_workspace_balances_frozen_nonnegative CHECK (frozen_micro_cny >= 0),
    CONSTRAINT fk_workspace_balances_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE ledger_entries (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL,
    direction VARCHAR(16) NOT NULL,
    amount_micro_cny BIGINT NOT NULL,
    balance_after_micro_cny BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL,
    idempotency_key VARCHAR(191) NOT NULL,
    request_id VARCHAR(191) NULL,
    api_key_id VARCHAR(32) NULL,
    api_key_name_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    model_id VARCHAR(191) NOT NULL DEFAULT '',
    model_display_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    price_version_id VARCHAR(32) NOT NULL DEFAULT '',
    unit_price_snapshot JSON NULL,
    metadata JSON NULL,
    created_at DATETIME(3) NOT NULL,
    CONSTRAINT chk_ledger_entries_amount_nonnegative CHECK (amount_micro_cny >= 0),
    UNIQUE KEY uk_ledger_entries_workspace_idempotency (workspace_id, idempotency_key),
    -- request_id dedupes non-null runtime settlement events.
    UNIQUE KEY uk_ledger_entries_request_id (request_id),
    KEY idx_ledger_entries_workspace_created (workspace_id, created_at),
    KEY idx_ledger_entries_api_key_id (api_key_id),
    CONSTRAINT fk_ledger_entries_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_ledger_entries_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE audit_logs (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NULL,
    actor_user_id VARCHAR(32) NULL,
    action VARCHAR(96) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    before_data JSON NULL,
    after_data JSON NULL,
    ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    KEY idx_audit_logs_workspace_created (workspace_id, created_at),
    KEY idx_audit_logs_actor_created (actor_user_id, created_at),
    KEY idx_audit_logs_resource (resource_type, resource_id),
    CONSTRAINT fk_audit_logs_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS workspace_balances;
DROP TABLE IF EXISTS api_key_model_whitelists;
DROP TABLE IF EXISTS workspace_model_permissions;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS workspace_invitations;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspace_runtime_accesses;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS account_identities;
DROP TABLE IF EXISTS users;
