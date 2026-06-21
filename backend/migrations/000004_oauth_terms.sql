-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN terms_accepted_at DATETIME(3) NULL AFTER email_verified_at;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE users
SET terms_accepted_at = created_at
WHERE terms_accepted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE account_identities
    ADD COLUMN display_name VARCHAR(120) NOT NULL DEFAULT '' AFTER email_verified,
    ADD COLUMN avatar_url VARCHAR(1024) NOT NULL DEFAULT '' AFTER display_name,
    ADD COLUMN linked_at DATETIME(3) NULL AFTER avatar_url;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE account_identities
    ADD UNIQUE KEY uk_account_identities_user_provider (user_id, provider);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE account_identities
    DROP INDEX uk_account_identities_user_provider,
    DROP COLUMN linked_at,
    DROP COLUMN avatar_url,
    DROP COLUMN display_name;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN terms_accepted_at;
-- +goose StatementEnd
