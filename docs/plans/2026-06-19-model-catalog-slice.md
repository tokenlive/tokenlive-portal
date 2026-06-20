# Model Catalog Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the backend model catalog tables, repository reads, and public model list/detail APIs.

**Architecture:** Extend the existing Go backend with a second goose migration, catalog domain models, repository read methods, and small `net/http` handlers registered by `portal-api`. The slice is read-only from Portal's public side; Admin publishing and Gateway sync remain outside scope.

**Tech Stack:** Go 1.24, standard `net/http`, GORM, MySQL, goose SQL migrations, `gorm.io/datatypes`, existing API error/request ID primitives.

---

## File Structure

Create:

- `backend/migrations/000002_model_catalog.sql`: catalog, i18n, price, and metrics tables.
- `backend/internal/repository/model_catalog.go`: public catalog query methods and DTOs.
- `backend/internal/repository/model_catalog_test.go`: optional MySQL integration tests.
- `backend/internal/api/public_models.go`: public model list/detail handlers.
- `backend/internal/api/public_models_test.go`: handler unit tests using fake reader.

Modify:

- `backend/internal/domain/models.go`: add model catalog domain structs and enums.
- `backend/internal/api/error.go`: add `model.not_found` and `model.invalid_query`.
- `backend/cmd/portal-api/main.go`: register public model routes.
- `docs/architecture/domain-model.md`: update only if implementation names differ from the existing model catalog section.

---

### Task 1: Add Model Catalog Migration And Domain Models

**Files:**

- Create: `backend/migrations/000002_model_catalog.sql`
- Modify: `backend/internal/domain/models.go`

- [ ] **Step 1: Create migration**

Create `backend/migrations/000002_model_catalog.sql`:

```sql
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
```

- [ ] **Step 2: Add domain structs and enums**

Append to `backend/internal/domain/models.go`:

```go
type ModelCatalogStatus string
type ModelCatalogVisibility string
type ModelPriceStatus string

const (
	ModelCatalogStatusAvailable ModelCatalogStatus = "available"
	ModelCatalogStatusPaused    ModelCatalogStatus = "paused"

	ModelCatalogVisibilityPublic  ModelCatalogVisibility = "public"
	ModelCatalogVisibilityPrivate ModelCatalogVisibility = "private"

	ModelPriceStatusActive   ModelPriceStatus = "active"
	ModelPriceStatusInactive ModelPriceStatus = "inactive"
)

type ModelCatalog struct {
	ModelID          string                 `gorm:"primaryKey;size:191"`
	Slug             string                 `gorm:"size:191;not null;uniqueIndex:uk_model_catalogs_slug"`
	Status           ModelCatalogStatus     `gorm:"size:32;not null;index:idx_model_catalogs_public_list,priority:2"`
	Visibility       ModelCatalogVisibility `gorm:"size:32;not null;index:idx_model_catalogs_public_list,priority:1"`
	LogoURL          string                 `gorm:"size:1024;not null;default:''"`
	ContextLength    *int64
	KnowledgeCutoff  *time.Time
	InputModalities  datatypes.JSON
	OutputModalities datatypes.JSON
	Capabilities     datatypes.JSON
	Featured         bool      `gorm:"not null;default:false;index:idx_model_catalogs_public_list,priority:3"`
	SortWeight       int64     `gorm:"not null;default:0;index:idx_model_catalogs_public_list,priority:4"`
	PublishedAt      *time.Time `gorm:"index:idx_model_catalogs_public_list,priority:5"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

type ModelCatalogI18n struct {
	ModelID          string         `gorm:"primaryKey;size:191"`
	Locale           string         `gorm:"primaryKey;size:16"`
	DisplayName      string         `gorm:"size:255;not null"`
	ShortDescription string         `gorm:"size:512;not null;default:''"`
	LongDescription  string         `gorm:"type:text"`
	SEOTitle         string         `gorm:"size:255;not null;default:''"`
	SEODescription   string         `gorm:"size:512;not null;default:''"`
	Tags             datatypes.JSON
	UpdatedAt        time.Time `gorm:"not null"`
}

type ModelPriceVersion struct {
	ID                             string           `gorm:"primaryKey;size:32"`
	ModelID                        string           `gorm:"size:191;not null;uniqueIndex:uk_model_price_versions_model_effective,priority:1;index:idx_model_price_versions_current,priority:1"`
	Currency                       string           `gorm:"size:8;not null"`
	InputMicroCNYPer1MTokens       int64            `gorm:"not null"`
	OutputMicroCNYPer1MTokens      int64            `gorm:"not null"`
	CacheReadMicroCNYPer1MTokens  *int64
	EffectiveFrom                 time.Time        `gorm:"not null;uniqueIndex:uk_model_price_versions_model_effective,priority:2;index:idx_model_price_versions_current,priority:3"`
	EffectiveUntil                *time.Time       `gorm:"index:idx_model_price_versions_current,priority:4"`
	Status                         ModelPriceStatus `gorm:"size:32;not null;index:idx_model_price_versions_current,priority:2"`
	PublishedByUserID             *string          `gorm:"size:32"`
	PublishedAt                   time.Time        `gorm:"not null"`
}

type ModelServiceMetric struct {
	ModelID       string     `gorm:"primaryKey;size:191"`
	Window        string     `gorm:"primaryKey;size:16"`
	Availability  *float64   `gorm:"type:decimal(8,6)"`
	TTFTP50MS     *int64
	TTFTP95MS     *int64
	ResponseSpeed *float64   `gorm:"type:decimal(18,6)"`
	SuccessRate   *float64   `gorm:"type:decimal(8,6)"`
	SampleCount   int64      `gorm:"not null;default:0"`
	UpdatedAt     time.Time  `gorm:"not null"`
}
```

Ensure imports already include `time` and `gorm.io/datatypes`; add only if missing.

- [ ] **Step 3: Format and compile domain**

Run:

```bash
cd backend
gofmt -w internal/domain/models.go
GOCACHE=/private/tmp/go-build-cache go test ./internal/domain
```

Expected: PASS.

---

### Task 2: Add Model Catalog Repository

**Files:**

- Create: `backend/internal/repository/model_catalog.go`
- Test: `backend/internal/repository/model_catalog_test.go`

- [ ] **Step 1: Add repository DTOs and methods**

Create `backend/internal/repository/model_catalog.go` with:

- `ErrModelNotFound`
- `DefaultLocale = "zh-CN"`
- `FallbackLocale = "en"`
- `MinPublicMetricSampleCount = int64(100)`
- `ModelCatalogListOptions`
- `PublicModel`
- `PublicModelDetail`
- `PublicModelPrice`
- `PublicModelMetric`
- `ListPublicModels(ctx, opts)`
- `GetPublicModelBySlug(ctx, slug, locale)`

Implementation requirements:

- Public filter: `visibility = public`, `status = available`, `published_at IS NOT NULL`.
- Locale normalization: empty -> `zh-CN`; only `zh-CN` and `en` accepted by repository, unknown -> `zh-CN`.
- Locale fallback: requested locale first, `en` second.
- Limit default 50, max 100, negative offset -> 0.
- Current price: active price where `effective_from <= now` and `effective_until IS NULL OR effective_until > now`, newest `effective_from` wins.
- Metrics: include only rows with `sample_count >= 100`.
- Detail returns `ErrModelNotFound` for missing private/unpublished slug.

- [ ] **Step 2: Add repository integration tests**

Create `backend/internal/repository/model_catalog_test.go`.

Tests should use `testDB(t)` and `uniqueSuffix(t)` from existing repository test helpers.

Add tests:

1. `TestListPublicModelsFiltersAndLocaleFallback`
   - Insert one public available model, one private model, one paused model.
   - Insert only `en` i18n for public model.
   - Query `locale=zh-CN`.
   - Assert only public available model returned and display name falls back to English.

2. `TestGetPublicModelBySlugIncludesCurrentPriceAndMetrics`
   - Insert public available model.
   - Insert expired, future, and current active price.
   - Insert metrics with one below threshold and one above threshold.
   - Assert detail uses current price and only above-threshold metric.

3. `TestGetPublicModelBySlugReturnsNotFoundForPrivate`
   - Insert private model.
   - Assert `errors.Is(err, ErrModelNotFound)`.

All inserted slugs/model IDs must use unique suffixes for repeatability.

- [ ] **Step 3: Run repository tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/repository -count=1 -v
```

Expected without `PORTAL_TEST_DATABASE_DSN`: PASS with DB-backed tests skipped.

---

### Task 3: Add Public Model API Handlers

**Files:**

- Modify: `backend/internal/api/error.go`
- Create: `backend/internal/api/public_models.go`
- Test: `backend/internal/api/public_models_test.go`

- [ ] **Step 1: Add API errors**

Modify `backend/internal/api/error.go`:

```go
CodeModelNotFound     ErrorCode = "model.not_found"
CodeModelInvalidQuery ErrorCode = "model.invalid_query"
```

Add app errors:

```go
ErrModelNotFound     = AppError{Code: CodeModelNotFound, Message: "Model not found", HTTPStatus: http.StatusNotFound}
ErrModelInvalidQuery = AppError{Code: CodeModelInvalidQuery, Message: "Invalid model query", HTTPStatus: http.StatusBadRequest}
```

- [ ] **Step 2: Add handler tests first**

Create `backend/internal/api/public_models_test.go` with fake reader tests for:

- List returns JSON `data` and `pagination`.
- Detail returns one model.
- Invalid limit returns `model.invalid_query`.
- Not found maps repository `ErrModelNotFound` to `model.not_found`.

Use `httptest.NewRecorder`, wrap requests with `RequestID`, and assert JSON fields.

- [ ] **Step 3: Implement handlers**

Create `backend/internal/api/public_models.go`:

- Define `PublicModelReader` interface.
- Define `PublicModelHandler`.
- Implement `RegisterPublicModelRoutes(mux *http.ServeMux, reader PublicModelReader)`.
- Register:
  - `GET /api/public/models`
  - `GET /api/public/models/{slug}`
- Parse `locale`, `featured`, `limit`, `offset`.
- Reject malformed bool/int query values with `ErrModelInvalidQuery`.
- Use existing `WriteError`.
- JSON encode repository DTOs directly or with small response structs.

- [ ] **Step 4: Run API tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/api
```

Expected: PASS.

---

### Task 4: Register Routes In portal-api And Verify

**Files:**

- Modify: `backend/cmd/portal-api/main.go`

- [ ] **Step 1: Wire repository when database DSN is present**

Modify `portal-api`:

- Load config as today.
- Always register `/healthz`.
- If `PORTAL_DATABASE_DSN` is non-empty:
  - open DB with `database.Open`
  - create `repository.New(db)`
  - register public model routes.
- If DSN is empty:
  - log that public model routes are disabled.
  - do not register model routes.

This keeps local `/healthz` runnable without DB while enabling real catalog routes in configured environments.

- [ ] **Step 2: Run verification**

Run:

```bash
cd backend
gofmt -w cmd/portal-api/main.go internal/api internal/repository internal/domain
GOCACHE=/private/tmp/go-build-cache go test ./...
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

Expected: PASS.

---

## Final Verification

- [ ] Run all tests:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./...
```

- [ ] Build both commands:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

- [ ] Confirm no build binaries remain under `backend/`:

```bash
find backend -maxdepth 2 -type f -perm +111 -print
```

Expected: no output.

- [ ] Check marker strings:

```bash
rg -n 'TB[D]|TO[D]O|place''holder|fill[ ]in|implement[ ]later' backend docs
```

Expected: only intentional ICP placeholder lines in product docs, if any.

