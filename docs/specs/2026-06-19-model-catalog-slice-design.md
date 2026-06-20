# Model Catalog Slice Design

Date: 2026-06-19

## 1. Goal

Build the first public model marketplace backend slice for TokenLive Portal.

This slice adds the data model and read APIs required for public model discovery:

- Published model catalog
- Localized model content
- Immutable public price versions
- Aggregated service metrics
- Public model list API
- Public model detail API

This slice does not implement Admin publishing, frontend pages, Gateway synchronization, or OpenAPI generation.

## 2. Scope

### Included

Database tables:

- `model_catalogs`
- `model_catalog_i18n`
- `model_price_versions`
- `model_service_metrics`

Backend code:

- GORM domain models and enums for the catalog tables.
- Repository methods for public model list and detail reads.
- Repository methods for current price and service metric lookup.
- Public HTTP handlers:
  - `GET /api/public/models`
  - `GET /api/public/models/{slug}`
- Locale handling:
  - Accept `locale=zh-CN|en`.
  - Default to `zh-CN`.
  - Fall back to `en` if requested locale content is missing.
- Tests:
  - API handler unit tests using fake repository.
  - Repository integration tests that skip when `PORTAL_TEST_DATABASE_DSN` is unset.

### Excluded

Not included:

- Admin model catalog editing or publishing APIs.
- Draft catalog content.
- Catalog revision history.
- Frontend marketplace pages.
- OpenAPI generation.
- Gateway availability synchronization.
- Search indexing.
- Complex filtering and ranking.
- User favorites.
- Model comparison pages.

## 3. Public API Behavior

### `GET /api/public/models`

Returns public, published models only.

Query parameters:

- `locale`: optional, defaults to `zh-CN`.
- `featured`: optional boolean. When true, returns featured models only.
- `limit`: optional integer, defaults to 50, max 100.
- `offset`: optional integer, defaults to 0.

Sort order:

1. `featured DESC`
2. `sort_weight DESC`
3. `published_at DESC`
4. `model_id ASC`

Response shape:

```json
{
  "data": [
    {
      "model_id": "openai/gpt-5",
      "slug": "openai-gpt-5",
      "status": "available",
      "display_name": "GPT-5",
      "short_description": "Flagship reasoning model.",
      "tags": ["reasoning", "coding"],
      "context_length": 128000,
      "input_modalities": ["text"],
      "output_modalities": ["text"],
      "capabilities": ["chat_completion", "responses"],
      "featured": true,
      "price": {
        "currency": "CNY",
        "input_micro_cny_per_1m_tokens": 1000000,
        "output_micro_cny_per_1m_tokens": 5000000,
        "cache_read_micro_cny_per_1m_tokens": 100000
      },
      "metrics": {
        "window": "24h",
        "availability": 0.999,
        "ttft_p50_ms": 650,
        "ttft_p95_ms": 1800,
        "success_rate": 0.995,
        "sample_count": 1234,
        "updated_at": "2026-06-19T00:00:00Z"
      }
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0
  }
}
```

The API may omit `price` or `metrics` when no active price or displayable metrics exist.

### `GET /api/public/models/{slug}`

Returns one public, published model by slug.

Query parameters:

- `locale`: optional, defaults to `zh-CN`.

Response includes all list fields plus:

- `long_description`
- `seo_title`
- `seo_description`
- `knowledge_cutoff`
- `logo_url`
- `service_metrics` for both `24h` and `7d` windows when available

Returns `404` with `model.not_found` when the slug is not public and published.

## 4. Schema Design

### `model_catalogs`

Current published model catalog row.

Key columns:

- `model_id VARCHAR(191) PRIMARY KEY`
- `slug VARCHAR(191) NOT NULL UNIQUE`
- `status VARCHAR(32) NOT NULL`
- `visibility VARCHAR(32) NOT NULL`
- `logo_url VARCHAR(1024) NOT NULL DEFAULT ''`
- `context_length BIGINT NULL`
- `knowledge_cutoff DATE NULL`
- `input_modalities JSON NULL`
- `output_modalities JSON NULL`
- `capabilities JSON NULL`
- `featured BOOLEAN NOT NULL DEFAULT FALSE`
- `sort_weight BIGINT NOT NULL DEFAULT 0`
- `published_at DATETIME(3) NULL`
- `created_at DATETIME(3) NOT NULL`
- `updated_at DATETIME(3) NOT NULL`

Rules:

- Public API reads only `visibility = 'public'`.
- Public API reads only published/available statuses.
- Provider and Endpoint details are not stored here.

### `model_catalog_i18n`

Localized content for published catalog rows.

Key columns:

- `model_id VARCHAR(191) NOT NULL`
- `locale VARCHAR(16) NOT NULL`
- `display_name VARCHAR(255) NOT NULL`
- `short_description VARCHAR(512) NOT NULL DEFAULT ''`
- `long_description TEXT NULL`
- `seo_title VARCHAR(255) NOT NULL DEFAULT ''`
- `seo_description VARCHAR(512) NOT NULL DEFAULT ''`
- `tags JSON NULL`
- `updated_at DATETIME(3) NOT NULL`

Primary key:

- `(model_id, locale)`

### `model_price_versions`

Immutable public model prices.

Key columns:

- `id VARCHAR(32) PRIMARY KEY`
- `model_id VARCHAR(191) NOT NULL`
- `currency VARCHAR(8) NOT NULL`
- `input_micro_cny_per_1m_tokens BIGINT NOT NULL`
- `output_micro_cny_per_1m_tokens BIGINT NOT NULL`
- `cache_read_micro_cny_per_1m_tokens BIGINT NULL`
- `effective_from DATETIME(3) NOT NULL`
- `effective_until DATETIME(3) NULL`
- `status VARCHAR(32) NOT NULL`
- `published_by_user_id VARCHAR(32) NULL`
- `published_at DATETIME(3) NOT NULL`

Rules:

- Prices are immutable once published.
- Public APIs use the currently active version:
  - `status = 'active'`
  - `effective_from <= now`
  - `effective_until IS NULL OR effective_until > now`
- Amount columns must be nonnegative.

### `model_service_metrics`

Aggregated display metrics.

Key columns:

- `model_id VARCHAR(191) NOT NULL`
- `window VARCHAR(16) NOT NULL`
- `availability DECIMAL(8,6) NULL`
- `ttft_p50_ms BIGINT NULL`
- `ttft_p95_ms BIGINT NULL`
- `response_speed DECIMAL(18,6) NULL`
- `success_rate DECIMAL(8,6) NULL`
- `sample_count BIGINT NOT NULL DEFAULT 0`
- `updated_at DATETIME(3) NOT NULL`

Primary key:

- `(model_id, window)`

Rules:

- Public API only returns metrics when `sample_count` meets the repository display threshold.
- First threshold is `100` samples.
- Provider and Endpoint details are not exposed.

## 5. Repository Design

Add catalog methods to the existing `repository.Repositories`.

Recommended method shape:

```go
type ModelCatalogListOptions struct {
    Locale   string
    Featured *bool
    Limit    int
    Offset   int
}

func (r *Repositories) ListPublicModels(ctx context.Context, opts ModelCatalogListOptions) ([]PublicModel, error)

func (r *Repositories) GetPublicModelBySlug(ctx context.Context, slug string, locale string) (PublicModelDetail, error)
```

Repository responsibilities:

- Clamp `limit` to max 100.
- Normalize locale.
- Load localized content with fallback.
- Attach current active price when present.
- Attach metrics only when `sample_count >= 100`.
- Return `ErrModelNotFound` for missing private/unpublished models.

## 6. HTTP Handler Design

Add a small public model handler under `internal/api`.

Recommended files:

- `backend/internal/api/public_models.go`
- `backend/internal/api/public_models_test.go`

Handler dependencies should use an interface, not concrete GORM repository, so handler tests can use fakes.

```go
type PublicModelReader interface {
    ListPublicModels(ctx context.Context, opts repository.ModelCatalogListOptions) ([]repository.PublicModel, error)
    GetPublicModelBySlug(ctx context.Context, slug string, locale string) (repository.PublicModelDetail, error)
}
```

Routing can remain simple in this slice:

- `RegisterPublicModelRoutes(mux *http.ServeMux, reader PublicModelReader)`

Use `http.ServeMux` path patterns compatible with Go 1.24.

## 7. Error Handling

Add model catalog errors:

- `model.not_found`
- `model.invalid_query`

Rules:

- Invalid query parameters return `400`.
- Missing public model returns `404`.
- All error responses use the existing API error envelope.
- All responses carry request ID through existing middleware.

## 8. Testing Strategy

Unit tests:

- Handler list response with fake reader.
- Handler detail response with fake reader.
- Handler invalid limit returns `model.invalid_query`.
- Handler not-found returns `model.not_found`.

Repository integration tests:

- Public list excludes private/unpublished models.
- Locale fallback works.
- Current price selection works.
- Expired/future prices are ignored.
- Metrics below sample threshold are omitted.

Repository integration tests skip cleanly if `PORTAL_TEST_DATABASE_DSN` is unset.

## 9. Acceptance Criteria

This slice is complete when:

- Migration `000002_model_catalog.sql` exists.
- Domain models exist for all four catalog tables.
- Repository read methods exist and are tested.
- Public model handlers exist and are unit tested.
- `portal-api` registers public model routes.
- `go test ./...` passes.
- `go build` for `portal-api` and `portal-worker` passes.
- No Admin/Gateway/frontend scope is added.

