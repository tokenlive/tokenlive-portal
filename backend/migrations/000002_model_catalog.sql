-- +goose Up
CREATE TABLE model_catalogs (
    model_id VARCHAR(191) PRIMARY KEY,
    slug VARCHAR(191) NOT NULL,
    status VARCHAR(32) NOT NULL,
    visibility VARCHAR(32) NOT NULL,
    logo_url VARCHAR(1024) NOT NULL DEFAULT '',
    context_length BIGINT NULL,
    knowledge_cutoff DATE NULL,
    input_modalities JSON NULL,
    output_modalities JSON NULL,
    capabilities JSON NULL,
    featured BOOLEAN NOT NULL DEFAULT FALSE,
    sort_weight BIGINT NOT NULL DEFAULT 0,
    published_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_model_catalogs_slug (slug),
    KEY idx_model_catalogs_public_list (visibility, status, featured, sort_weight, published_at),
    CONSTRAINT chk_model_catalogs_context_length_nonnegative CHECK (context_length IS NULL OR context_length >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE model_catalog_i18n (
    model_id VARCHAR(191) NOT NULL,
    locale VARCHAR(16) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    short_description VARCHAR(512) NOT NULL DEFAULT '',
    long_description TEXT NULL,
    seo_title VARCHAR(255) NOT NULL DEFAULT '',
    seo_description VARCHAR(512) NOT NULL DEFAULT '',
    tags JSON NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (model_id, locale),
    CONSTRAINT fk_model_catalog_i18n_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE model_price_versions (
    id VARCHAR(32) PRIMARY KEY,
    model_id VARCHAR(191) NOT NULL,
    currency VARCHAR(8) NOT NULL,
    input_micro_cny_per_1m_tokens BIGINT NOT NULL,
    output_micro_cny_per_1m_tokens BIGINT NOT NULL,
    cache_read_micro_cny_per_1m_tokens BIGINT NULL,
    effective_from DATETIME(3) NOT NULL,
    effective_until DATETIME(3) NULL,
    status VARCHAR(32) NOT NULL,
    published_by_user_id VARCHAR(32) NULL,
    published_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_model_price_versions_model_effective (model_id, effective_from),
    KEY idx_model_price_versions_current (model_id, status, effective_from, effective_until),
    CONSTRAINT fk_model_price_versions_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id),
    CONSTRAINT fk_model_price_versions_published_by FOREIGN KEY (published_by_user_id) REFERENCES users(id),
    CONSTRAINT chk_model_price_versions_input_nonnegative CHECK (input_micro_cny_per_1m_tokens >= 0),
    CONSTRAINT chk_model_price_versions_output_nonnegative CHECK (output_micro_cny_per_1m_tokens >= 0),
    CONSTRAINT chk_model_price_versions_cache_nonnegative CHECK (cache_read_micro_cny_per_1m_tokens IS NULL OR cache_read_micro_cny_per_1m_tokens >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE model_service_metrics (
    model_id VARCHAR(191) NOT NULL,
    window VARCHAR(16) NOT NULL,
    availability DECIMAL(8,6) NULL,
    ttft_p50_ms BIGINT NULL,
    ttft_p95_ms BIGINT NULL,
    response_speed DECIMAL(18,6) NULL,
    success_rate DECIMAL(8,6) NULL,
    sample_count BIGINT NOT NULL DEFAULT 0,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (model_id, window),
    CONSTRAINT fk_model_service_metrics_model FOREIGN KEY (model_id) REFERENCES model_catalogs(model_id),
    CONSTRAINT chk_model_service_metrics_sample_nonnegative CHECK (sample_count >= 0),
    CONSTRAINT chk_model_service_metrics_ttft_p50_nonnegative CHECK (ttft_p50_ms IS NULL OR ttft_p50_ms >= 0),
    CONSTRAINT chk_model_service_metrics_ttft_p95_nonnegative CHECK (ttft_p95_ms IS NULL OR ttft_p95_ms >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS model_service_metrics;
DROP TABLE IF EXISTS model_price_versions;
DROP TABLE IF EXISTS model_catalog_i18n;
DROP TABLE IF EXISTS model_catalogs;
