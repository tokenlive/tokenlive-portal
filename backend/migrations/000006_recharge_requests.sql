-- +goose Up
CREATE TABLE recharge_requests (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    requested_by_user_id VARCHAR(32) NOT NULL,
    amount_micro_cny BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL,
    status VARCHAR(32) NOT NULL,
    payment_method VARCHAR(64) NOT NULL DEFAULT '',
    contact VARCHAR(320) NOT NULL DEFAULT '',
    note TEXT NULL,
    admin_note TEXT NULL,
    ledger_entry_id VARCHAR(32) NULL,
    reviewed_by_user_id VARCHAR(32) NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    CONSTRAINT chk_recharge_requests_amount_positive CHECK (amount_micro_cny > 0),
    KEY idx_recharge_requests_workspace_created (workspace_id, created_at),
    KEY idx_recharge_requests_requested_by_user_id (requested_by_user_id),
    KEY idx_recharge_requests_status (status),
    CONSTRAINT fk_recharge_requests_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_recharge_requests_requested_by FOREIGN KEY (requested_by_user_id) REFERENCES users(id),
    CONSTRAINT fk_recharge_requests_ledger_entry FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id),
    CONSTRAINT fk_recharge_requests_reviewed_by FOREIGN KEY (reviewed_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS recharge_requests;
