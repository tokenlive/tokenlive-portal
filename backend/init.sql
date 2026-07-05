-- TokenLive Portal current schema snapshot.
-- Use this file to initialize a fresh database to the latest schema.
-- Goose migrations in backend/migrations remain the source for incremental upgrades.

CREATE TABLE users (
    id VARCHAR(32) PRIMARY KEY COMMENT '用户ID',
    display_name VARCHAR(120) NOT NULL DEFAULT '' COMMENT '用户展示名称',
    primary_email VARCHAR(320) NULL COMMENT '主邮箱地址',
    email_verified_at DATETIME(3) NULL COMMENT '邮箱验证时间',
    terms_accepted_at DATETIME(3) NULL COMMENT '服务条款接受时间',
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '头像URL',
    status VARCHAR(32) NOT NULL COMMENT '用户状态',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '软删除时间',
    UNIQUE KEY uk_users_primary_email (primary_email),
    KEY idx_users_status (status),
    KEY idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户账号表';

CREATE TABLE account_identities (
    id VARCHAR(32) PRIMARY KEY COMMENT '账号身份ID',
    user_id VARCHAR(32) NOT NULL COMMENT '关联用户ID',
    provider VARCHAR(32) NOT NULL COMMENT '身份提供方',
    provider_subject VARCHAR(191) NOT NULL COMMENT '提供方用户唯一标识',
    email VARCHAR(320) NOT NULL DEFAULT '' COMMENT '提供方邮箱',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE COMMENT '提供方邮箱是否已验证',
    display_name VARCHAR(120) NOT NULL DEFAULT '' COMMENT '提供方展示名称',
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '提供方头像URL',
    linked_at DATETIME(3) NULL COMMENT '绑定时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    UNIQUE KEY uk_account_identities_provider_subject (provider, provider_subject),
    UNIQUE KEY uk_account_identities_user_provider (user_id, provider),
    KEY idx_account_identities_user_id (user_id),
    CONSTRAINT fk_account_identities_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='第三方账号身份绑定表';

CREATE TABLE model_catalogs (
    model_id VARCHAR(191) PRIMARY KEY COMMENT '模型ID',
    slug VARCHAR(191) NOT NULL COMMENT '模型URL标识',
    status VARCHAR(32) NOT NULL COMMENT '模型状态',
    visibility VARCHAR(32) NOT NULL COMMENT '可见性',
    logo_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '模型Logo URL',
    context_length BIGINT NULL COMMENT '上下文长度',
    knowledge_cutoff DATE NULL COMMENT '知识截止日期',
    input_modalities JSON NULL COMMENT '支持的输入模态',
    output_modalities JSON NULL COMMENT '支持的输出模态',
    capabilities JSON NULL COMMENT '模型能力标签',
    featured BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否精选',
    sort_weight BIGINT NOT NULL DEFAULT 0 COMMENT '排序权重',
    published_at DATETIME(3) NULL COMMENT '发布时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    UNIQUE KEY uk_model_catalogs_slug (slug),
    KEY idx_model_catalogs_public_list (visibility, status, featured, sort_weight, published_at),
    CONSTRAINT chk_model_catalogs_context_length_nonnegative CHECK (context_length IS NULL OR context_length >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模型目录主表';

CREATE TABLE model_catalog_i18n (
    model_id VARCHAR(191) NOT NULL COMMENT '模型ID',
    locale VARCHAR(16) NOT NULL COMMENT '语言区域',
    display_name VARCHAR(255) NOT NULL COMMENT '模型展示名称',
    short_description VARCHAR(512) NOT NULL DEFAULT '' COMMENT '短描述',
    long_description TEXT NULL COMMENT '长描述',
    seo_title VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'SEO标题',
    seo_description VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'SEO描述',
    tags JSON NULL COMMENT '展示标签',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    PRIMARY KEY (model_id, locale),
    CONSTRAINT fk_model_catalog_i18n_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模型目录多语言内容表';

CREATE TABLE model_price_versions (
    id VARCHAR(32) PRIMARY KEY COMMENT '价格版本ID',
    model_id VARCHAR(191) NOT NULL COMMENT '模型ID',
    currency VARCHAR(8) NOT NULL COMMENT '币种',
    input_price DECIMAL(18,9) NOT NULL COMMENT '输入价格，CNY/1M tokens',
    output_price DECIMAL(18,9) NOT NULL COMMENT '输出价格，CNY/1M tokens',
    cached_price DECIMAL(18,9) NULL COMMENT '缓存命中价格，CNY/1M tokens',
    cache_creation_price DECIMAL(18,9) NULL COMMENT '缓存创建价格，CNY/1M tokens',
    effective_from DATETIME(3) NOT NULL COMMENT '生效时间',
    effective_until DATETIME(3) NULL COMMENT '失效时间',
    status VARCHAR(32) NOT NULL COMMENT '价格状态',
    published_at DATETIME(3) NOT NULL COMMENT '发布时间',
    UNIQUE KEY uk_model_price_versions_model_effective (model_id, effective_from),
    KEY idx_model_price_versions_current (model_id, status, effective_from, effective_until),
    CONSTRAINT fk_model_price_versions_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id),
    CONSTRAINT chk_model_price_versions_input_nonnegative CHECK (input_price >= 0),
    CONSTRAINT chk_model_price_versions_output_nonnegative CHECK (output_price >= 0),
    CONSTRAINT chk_model_price_versions_cache_nonnegative CHECK (cached_price IS NULL OR cached_price >= 0),
    CONSTRAINT chk_model_price_versions_cache_creation_nonnegative CHECK (cache_creation_price IS NULL OR cache_creation_price >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模型价格版本表';

CREATE TABLE model_service_metrics (
    model_id VARCHAR(191) NOT NULL COMMENT '模型ID',
    window VARCHAR(16) NOT NULL COMMENT '统计窗口',
    availability DECIMAL(8,6) NULL COMMENT '可用率',
    ttft_p50_ms BIGINT NULL COMMENT '首token延迟P50毫秒',
    ttft_p95_ms BIGINT NULL COMMENT '首token延迟P95毫秒',
    response_speed DECIMAL(18,6) NULL COMMENT '响应速度',
    success_rate DECIMAL(8,6) NULL COMMENT '成功率',
    sample_count BIGINT NOT NULL DEFAULT 0 COMMENT '样本数量',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    PRIMARY KEY (model_id, window),
    CONSTRAINT fk_model_service_metrics_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id),
    CONSTRAINT chk_model_service_metrics_sample_nonnegative CHECK (sample_count >= 0),
    CONSTRAINT chk_model_service_metrics_ttft_p50_nonnegative CHECK (ttft_p50_ms IS NULL OR ttft_p50_ms >= 0),
    CONSTRAINT chk_model_service_metrics_ttft_p95_nonnegative CHECK (ttft_p95_ms IS NULL OR ttft_p95_ms >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模型服务指标表';

CREATE TABLE workspaces (
    id VARCHAR(32) PRIMARY KEY COMMENT '工作空间ID',
    name VARCHAR(160) NOT NULL COMMENT '工作空间名称',
    slug VARCHAR(160) NOT NULL COMMENT '工作空间URL标识',
    owner_user_id VARCHAR(32) NOT NULL COMMENT '所有者用户ID',
    tenant_code VARCHAR(64) NULL DEFAULT NULL COMMENT '关联的Admin租户唯一英文编码',
    status VARCHAR(32) NOT NULL COMMENT '工作空间状态',
    trial_granted_at DATETIME(3) NULL COMMENT '试用金发放时间',
    created_by_user_id VARCHAR(32) NOT NULL COMMENT '创建人用户ID',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '软删除时间',
    UNIQUE KEY uk_workspaces_slug (slug),
    KEY idx_workspaces_owner_user_id (owner_user_id),
    KEY idx_workspaces_created_by_user_id (created_by_user_id),
    KEY idx_workspaces_deleted_at (deleted_at),
    CONSTRAINT fk_workspaces_owner_user FOREIGN KEY (owner_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspaces_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间表';

CREATE TABLE workspace_members (
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    user_id VARCHAR(32) NOT NULL COMMENT '成员用户ID',
    role VARCHAR(32) NOT NULL COMMENT '成员角色',
    status VARCHAR(32) NOT NULL COMMENT '成员状态',
    joined_at DATETIME(3) NULL COMMENT '加入时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    PRIMARY KEY (workspace_id, user_id),
    KEY idx_workspace_members_user_id (user_id),
    CONSTRAINT fk_workspace_members_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间成员表';

CREATE TABLE workspace_invitations (
    id VARCHAR(32) PRIMARY KEY COMMENT '邀请ID',
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    email VARCHAR(320) NOT NULL COMMENT '被邀请邮箱',
    role VARCHAR(32) NOT NULL COMMENT '邀请角色',
    token_hash VARCHAR(128) NOT NULL COMMENT '邀请令牌哈希',
    status VARCHAR(32) NOT NULL COMMENT '邀请状态',
    invited_by_user_id VARCHAR(32) NOT NULL COMMENT '邀请人用户ID',
    accepted_by_user_id VARCHAR(32) NULL COMMENT '接受邀请用户ID',
    expires_at DATETIME(3) NOT NULL COMMENT '过期时间',
    accepted_at DATETIME(3) NULL COMMENT '接受时间',
    revoked_at DATETIME(3) NULL COMMENT '撤销时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    UNIQUE KEY uk_workspace_invitations_token_hash (token_hash),
    KEY idx_workspace_invitations_workspace_email (workspace_id, email),
    KEY idx_workspace_invitations_status (status),
    CONSTRAINT fk_workspace_invitations_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_invitations_invited_by FOREIGN KEY (invited_by_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspace_invitations_accepted_by FOREIGN KEY (accepted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间邀请表';

CREATE TABLE api_keys (
    id VARCHAR(32) PRIMARY KEY COMMENT 'API Key ID',
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    name VARCHAR(160) NOT NULL COMMENT 'API Key 名称',
    key_prefix VARCHAR(32) NOT NULL COMMENT '密钥前缀',
    secret_last4 VARCHAR(8) NOT NULL COMMENT '密钥后四位',
    key_hash VARCHAR(128) NOT NULL COMMENT '密钥哈希',
    status VARCHAR(32) NOT NULL COMMENT '密钥状态',
    created_by_user_id VARCHAR(32) NOT NULL COMMENT '创建人用户ID',
    expires_at DATETIME(3) NULL COMMENT '过期时间',
    daily_limit_micro_cny BIGINT NULL COMMENT '每日限额',
    monthly_limit_micro_cny BIGINT NULL COMMENT '每月限额',
    last_used_at DATETIME(3) NULL COMMENT '最后使用时间',
    total_spend_micro_cny BIGINT NOT NULL DEFAULT 0 COMMENT '累计消费金额',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    revoked_at DATETIME(3) NULL COMMENT '撤销时间',
    UNIQUE KEY uk_api_keys_key_hash (key_hash),
    KEY idx_api_keys_workspace_id (workspace_id),
    KEY idx_api_keys_created_by_user_id (created_by_user_id),
    KEY idx_api_keys_status (status),
    CONSTRAINT fk_api_keys_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_api_keys_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间API密钥表';

CREATE TABLE workspace_model_permissions (
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    model_id VARCHAR(191) NOT NULL COMMENT '模型ID',
    source VARCHAR(32) NOT NULL COMMENT '授权来源',
    granted_by_user_id VARCHAR(32) NULL COMMENT '授权人用户ID',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    PRIMARY KEY (workspace_id, model_id),
    KEY idx_workspace_model_permissions_model_id (model_id),
    CONSTRAINT fk_workspace_model_permissions_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_model_permissions_granted_by FOREIGN KEY (granted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间模型授权表';

CREATE TABLE api_key_model_whitelists (
    api_key_id VARCHAR(32) NOT NULL COMMENT 'API Key ID',
    model_id VARCHAR(191) NOT NULL COMMENT '模型ID',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    PRIMARY KEY (api_key_id, model_id),
    KEY idx_api_key_model_whitelists_model_id (model_id),
    CONSTRAINT fk_api_key_model_whitelists_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='API密钥模型白名单表';

CREATE TABLE workspace_balances (
    workspace_id VARCHAR(32) PRIMARY KEY COMMENT '工作空间ID',
    available_micro_cny BIGINT NOT NULL DEFAULT 0 COMMENT '可用余额',
    frozen_micro_cny BIGINT NOT NULL DEFAULT 0 COMMENT '冻结余额',
    version BIGINT NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    CONSTRAINT chk_workspace_balances_available_nonnegative CHECK (available_micro_cny >= 0),
    CONSTRAINT chk_workspace_balances_frozen_nonnegative CHECK (frozen_micro_cny >= 0),
    CONSTRAINT fk_workspace_balances_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作空间余额表';

CREATE TABLE ledger_entries (
    id VARCHAR(32) PRIMARY KEY COMMENT '账务流水ID',
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    type VARCHAR(32) NOT NULL COMMENT '流水类型',
    direction VARCHAR(16) NOT NULL COMMENT '资金方向',
    amount_micro_cny BIGINT NOT NULL COMMENT '变动金额',
    balance_after_micro_cny BIGINT NOT NULL COMMENT '变动后余额',
    currency VARCHAR(8) NOT NULL COMMENT '货币',
    idempotency_key VARCHAR(191) NOT NULL COMMENT '幂等键',
    request_id VARCHAR(191) NULL COMMENT '网关请求ID',
    api_key_id VARCHAR(32) NULL COMMENT '关联API Key ID',
    api_key_name_snapshot VARCHAR(160) NOT NULL DEFAULT '' COMMENT 'API Key名称快照',
    model_id VARCHAR(191) NOT NULL DEFAULT '' COMMENT '模型ID',
    model_display_name_snapshot VARCHAR(255) NOT NULL DEFAULT '' COMMENT '模型名称快照',
    price_version_id VARCHAR(32) NOT NULL DEFAULT '' COMMENT '价格版本ID',
    unit_price_snapshot JSON NULL COMMENT '单价快照',
    metadata JSON NULL COMMENT '扩展元数据',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    CONSTRAINT chk_ledger_entries_amount_nonnegative CHECK (amount_micro_cny >= 0),
    UNIQUE KEY uk_ledger_entries_workspace_idempotency (workspace_id, idempotency_key),
    UNIQUE KEY uk_ledger_entries_request_id (request_id),
    KEY idx_ledger_entries_workspace_created (workspace_id, created_at),
    KEY idx_ledger_entries_api_key_id (api_key_id),
    CONSTRAINT fk_ledger_entries_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_ledger_entries_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账务流水表';

CREATE TABLE recharge_requests (
    id VARCHAR(32) PRIMARY KEY COMMENT '充值申请ID',
    workspace_id VARCHAR(32) NOT NULL COMMENT '工作空间ID',
    requested_by_user_id VARCHAR(32) NOT NULL COMMENT '申请人用户ID',
    amount_micro_cny BIGINT NOT NULL COMMENT '申请充值金额',
    currency VARCHAR(8) NOT NULL COMMENT '货币',
    status VARCHAR(32) NOT NULL COMMENT '申请状态',
    payment_method VARCHAR(64) NOT NULL DEFAULT '' COMMENT '付款方式',
    contact VARCHAR(320) NOT NULL DEFAULT '' COMMENT '联系信息',
    note TEXT NULL COMMENT '申请备注',
    admin_note TEXT NULL COMMENT '审核备注',
    ledger_entry_id VARCHAR(32) NULL COMMENT '入账流水ID',
    reviewed_by_user_id VARCHAR(32) NULL COMMENT '审核人用户ID',
    reviewed_at DATETIME(3) NULL COMMENT '审核时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL COMMENT '更新时间',
    CONSTRAINT chk_recharge_requests_amount_positive CHECK (amount_micro_cny > 0),
    KEY idx_recharge_requests_workspace_created (workspace_id, created_at),
    KEY idx_recharge_requests_requested_by_user_id (requested_by_user_id),
    KEY idx_recharge_requests_status (status),
    CONSTRAINT fk_recharge_requests_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_recharge_requests_requested_by FOREIGN KEY (requested_by_user_id) REFERENCES users(id),
    CONSTRAINT fk_recharge_requests_ledger_entry FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id),
    CONSTRAINT fk_recharge_requests_reviewed_by FOREIGN KEY (reviewed_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充值申请表';

CREATE TABLE audit_logs (
    id VARCHAR(32) PRIMARY KEY COMMENT '审计日志ID',
    workspace_id VARCHAR(32) NULL COMMENT '工作空间ID',
    actor_user_id VARCHAR(32) NULL COMMENT '操作人用户ID',
    action VARCHAR(96) NOT NULL COMMENT '操作动作',
    resource_type VARCHAR(64) NOT NULL COMMENT '资源类型',
    resource_id VARCHAR(64) NOT NULL COMMENT '资源ID',
    before_data JSON NULL COMMENT '变更前数据',
    after_data JSON NULL COMMENT '变更后数据',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    KEY idx_audit_logs_workspace_created (workspace_id, created_at),
    KEY idx_audit_logs_actor_created (actor_user_id, created_at),
    KEY idx_audit_logs_resource (resource_type, resource_id),
    CONSTRAINT fk_audit_logs_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审计日志表';

CREATE TABLE user_sessions (
    id VARCHAR(32) PRIMARY KEY COMMENT '会话ID',
    user_id VARCHAR(32) NOT NULL COMMENT '用户ID',
    token_hash VARCHAR(128) NOT NULL COMMENT '会话令牌哈希',
    status VARCHAR(32) NOT NULL COMMENT '会话状态',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '登录IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
    expires_at DATETIME(3) NOT NULL COMMENT '过期时间',
    last_seen_at DATETIME(3) NULL COMMENT '最后访问时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    revoked_at DATETIME(3) NULL COMMENT '撤销时间',
    UNIQUE KEY uk_user_sessions_token_hash (token_hash),
    KEY idx_user_sessions_user_status (user_id, status),
    KEY idx_user_sessions_expires_at (expires_at),
    CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户会话表';

CREATE TABLE email_verification_codes (
    id VARCHAR(32) PRIMARY KEY COMMENT '验证码ID',
    email VARCHAR(320) NOT NULL COMMENT '邮箱地址',
    purpose VARCHAR(32) NOT NULL COMMENT '验证码用途',
    code_hash VARCHAR(128) NOT NULL COMMENT '验证码哈希',
    status VARCHAR(32) NOT NULL COMMENT '验证码状态',
    attempt_count INT NOT NULL DEFAULT 0 COMMENT '验证尝试次数',
    expires_at DATETIME(3) NOT NULL COMMENT '过期时间',
    consumed_at DATETIME(3) NULL COMMENT '消费时间',
    created_at DATETIME(3) NOT NULL COMMENT '创建时间',
    KEY idx_email_codes_lookup (email, purpose, status, created_at),
    CONSTRAINT chk_email_codes_attempt_nonnegative CHECK (attempt_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮箱验证码表';
