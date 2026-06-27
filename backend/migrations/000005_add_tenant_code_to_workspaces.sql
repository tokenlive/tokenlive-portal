-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspaces
    ADD COLUMN tenant_code VARCHAR(64) NULL DEFAULT NULL COMMENT '关联的Admin租户唯一英文编码' AFTER owner_user_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspaces
    DROP COLUMN tenant_code;
-- +goose StatementEnd
