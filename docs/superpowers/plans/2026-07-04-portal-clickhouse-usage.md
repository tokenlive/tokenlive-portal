# Portal ClickHouse Usage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Portal Usage and Request Logs slice using Gateway ClickHouse `access_logs` as the source of truth.

**Architecture:** Gateway enriches ClickHouse access logs with Portal identity fields (`workspace_id`, `api_key_id`, `api_key_hash`) and keeps using the existing AccessLogFilter batcher plus Redis compensation path. Portal adds a separate read-only ClickHouse client and exposes Workspace-scoped Usage Summary and Request Logs APIs, then renders them in `/console/usage`.

**Tech Stack:** Go 1.25, ClickHouse Go v2, MySQL/GORM for Portal identity, Next.js 16, React 19, TypeScript, Node test runner, ESLint.

## Global Constraints

- Gateway must not call Portal during the request path.
- Portal must not depend on Admin APIs for user-facing usage.
- Redis `aigw:status:*` keys remain Admin/global operational telemetry only.
- No new Portal Workspace usage Redis counters are added in this slice.
- Portal Usage APIs must always scope by the current Workspace ID resolved from the Portal session.
- Frontend must never receive `api_key_hash`.
- Frontend must never receive plaintext API key.
- Prompt and output content remain out of scope and are not stored in the current Gateway access log table.
- Request log `limit` is capped at 100.
- ClickHouse usage is disabled unless `PORTAL_USAGE_CLICKHOUSE_ENABLED=true`.
- Query failure returns an unavailable response for Usage instead of a generic 500 when possible.

---

## File Structure

### Gateway Repository: `/Users/chenzhiguo/Projects/tokenlive-gateway`

- Modify `pkg/config/provider.go`: add `KeyID` and `KeyHash` to the runtime API key item contract.
- Modify `pkg/config/redis_provider.go`: read `key_id` from Redis and expose the computed `key_hash`.
- Modify `internal/service/apikey.go`: carry `KeyID` and `KeyHash` through `ApiKeyInfo`.
- Modify `internal/middleware/auth.go`: propagate API key identity as internal headers.
- Modify `pkg/core/context.go`: add `APIKeyID` and `APIKeyHash` to `GatewayContext`.
- Modify `pkg/core/engine_request.go`: copy API key identity headers into `GatewayContext`.
- Modify `pkg/filters/outbound/access_log.go`: add Portal identity fields to `AccessLogItem`, populate them from `GatewayContext`, and append them to ClickHouse batches.
- Modify `internal/repository/clickhouse.go`: add ClickHouse columns to `access_logs` DDL.
- Modify tests near the touched files.

### Portal Repository: `/Users/chenzhiguo/Projects/tokenlive-portal`

- Modify `backend/go.mod` and `backend/go.sum`: add `github.com/ClickHouse/clickhouse-go/v2`.
- Modify `backend/internal/config/config.go`: add `ClickHouseConfig` and `PORTAL_USAGE_CLICKHOUSE_ENABLED` parsing.
- Modify `backend/internal/config/config_test.go`: cover enabled, disabled, trimmed, and missing ClickHouse config.
- Create `backend/internal/usage/clickhouse.go`: ClickHouse read client, query interface, and disabled client.
- Create `backend/internal/usage/service.go`: Workspace-scoped usage response shaping.
- Create `backend/internal/usage/service_test.go`: service tests using fake readers.
- Modify `backend/internal/api/console_service.go`: add usage response structs and usage service dependency, or create usage-specific service interfaces in `backend/internal/api/usage.go`.
- Modify `backend/internal/api/console.go`: register `GET /api/usage/summary` and `GET /api/request-logs`.
- Modify `backend/internal/api/console_test.go`: handler auth, scoping, unavailable response, and response shape tests.
- Modify `backend/cmd/portal-api/main.go`: initialize ClickHouse usage reader and pass it into the API layer.
- Modify `backend/cmd/portal-api/main_test.go`: verify usage routes are registered and ClickHouse config is passed.
- Modify `web/src/types/api.ts`: add Usage Summary and Request Log types.
- Modify `web/src/lib/api.ts`: add `fetchUsageSummary()` and `fetchRequestLogs()`.
- Create `web/src/lib/usage-display.js`: formatting helpers.
- Create `web/tests/usage-display.test.mjs`: helper tests.
- Create `web/src/app/console/usage/page.tsx`: Usage page.
- Modify `web/src/app/console/layout.tsx`: add Usage nav item.

---

### Task 1: Gateway Runtime API Key Identity

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/provider.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/service/apikey.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/middleware/auth.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/core/context.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/core/engine_request.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider_api_key_test.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/service/apikey_hash_test.go`
- Test: add or extend `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/middleware/auth_test.go`

**Interfaces:**
- Consumes: Redis runtime hash fields `key_id`, `workspace_id`, `user_id`, `tenant`, `user_tenant`, `status`, `quota`, `expires_at`.
- Produces: `GatewayContext.APIKeyID`, `GatewayContext.APIKeyHash`, and existing `GatewayContext.WorkspaceID`.

- [ ] **Step 1: Write failing Redis provider test**

Add assertions to `TestRedisGatewayProviderGetApiKeyPrefersAPIKeyHashLookup`:

```go
if got.KeyID != "ak_1" {
	t.Fatalf("KeyID = %q, want ak_1", got.KeyID)
}
if got.KeyHash != keyHash {
	t.Fatalf("KeyHash = %q, want %q", got.KeyHash, keyHash)
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./pkg/config -run TestRedisGatewayProviderGetApiKeyPrefersAPIKeyHashLookup
```

Expected: FAIL because `HTTPApiKeyItem` has no `KeyID` / `KeyHash` fields.

- [ ] **Step 3: Add fields to provider contract**

In `pkg/config/provider.go`, update `HTTPApiKeyItem`:

```go
type HTTPApiKeyItem struct {
	APIKey      string `json:"api_key"`
	KeyID       string `json:"key_id"`
	KeyHash     string `json:"key_hash"`
	UserID      string `json:"user_id"`
	Tenant      string `json:"tenant"`
	WorkspaceID string `json:"workspace_id"`
	UserTenant  string `json:"user_tenant"`
	Status      int    `json:"status"`
	Quota       int64  `json:"quota"`
	ExpiresAt   int64  `json:"expires_at"`
}
```

- [ ] **Step 4: Populate fields in Redis provider**

In `pkg/config/redis_provider.go`, inside the hash lookup path after computing `keyHash`, set:

```go
item := &HTTPApiKeyItem{
	APIKey:      apiKey,
	KeyID:       fields["key_id"],
	KeyHash:     keyHash,
	UserID:      fields["user_id"],
	Tenant:      fields["tenant"],
	WorkspaceID: workspaceID,
	UserTenant:  userTenant,
	Status:      status,
	Quota:       quota,
	ExpiresAt:   expiresAt,
}
```

- [ ] **Step 5: Run provider test to verify GREEN**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./pkg/config -run TestRedisGatewayProviderGetApiKeyPrefersAPIKeyHashLookup
```

Expected: PASS.

- [ ] **Step 6: Write failing service/context propagation test**

In `internal/service/apikey_hash_test.go`, add assertions:

```go
if info.KeyID != "ak_1" {
	t.Fatalf("KeyID = %q, want ak_1", info.KeyID)
}
if info.KeyHash != keyHash {
	t.Fatalf("KeyHash = %q, want %q", info.KeyHash, keyHash)
}
```

- [ ] **Step 7: Run service test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./internal/service -run TestApiKeyServiceValidateAPIKeyHashKey
```

Expected: FAIL because `ApiKeyInfo` has no `KeyID` / `KeyHash`.

- [ ] **Step 8: Add identity fields to service and context**

In `internal/service/apikey.go`:

```go
type ApiKeyInfo struct {
	KeyID       string `json:"key_id"`
	KeyHash     string `json:"key_hash"`
	UserID      string `json:"user_id"`
	Tenant      string `json:"tenant"`
	WorkspaceID string `json:"workspace_id"`
	UserTenant  string `json:"user_tenant"`
	Status      int    `json:"status"`
	Quota       int64  `json:"quota"`
	ExpiresAt   int64  `json:"expires_at"`
}
```

And when building `info`:

```go
info := &ApiKeyInfo{
	KeyID:       item.KeyID,
	KeyHash:     item.KeyHash,
	UserID:      item.UserID,
	Tenant:      item.Tenant,
	WorkspaceID: item.WorkspaceID,
	UserTenant:  item.UserTenant,
	Status:      item.Status,
	Quota:       item.Quota,
	ExpiresAt:   item.ExpiresAt,
}
```

In `pkg/core/context.go`, add fields after `APIKey`:

```go
APIKeyID   string
APIKeyHash string
```

Update `VerifyKey` in `internal/service/apikey.go` to return identity fields:

```go
func (s *ApiKeyService) VerifyKey(ctx context.Context, apiKey string) (userID, tenant, workspaceID, userTenant, keyID, keyHash string, err error) {
	info, err := s.ValidateKey(ctx, apiKey)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	return info.UserID, info.Tenant, info.WorkspaceID, info.UserTenant, info.KeyID, info.KeyHash, nil
}
```

Update `internal/middleware/auth.go`:

```go
type ApiKeyValidator interface {
	VerifyKey(ctx context.Context, apiKey string) (userID, tenant, workspaceID, userTenant, keyID, keyHash string, err error)
}
```

After successful validation, inject:

```go
if keyID != "" {
	c.Request.Header.Set("X-API-Key-ID", keyID)
	c.Set("api_key_id", keyID)
}
if keyHash != "" {
	c.Request.Header.Set("X-API-Key-Hash", keyHash)
	c.Set("api_key_hash", keyHash)
}
```

In `pkg/core/engine_request.go`, read the internal headers alongside the existing identity headers:

```go
gctx.APIKeyID = gctx.Request.Header.Get("X-API-Key-ID")
gctx.APIKeyHash = gctx.Request.Header.Get("X-API-Key-Hash")
```

Update service and middleware tests that implement `VerifyKey` fakes to use the extended return signature.

- [ ] **Step 9: Run gateway identity tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./pkg/config ./internal/service ./pkg/core
```

Expected: PASS.

---

### Task 2: Gateway ClickHouse Access Log Contract

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/repository/clickhouse.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/filters/outbound/access_log.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/filters/outbound/access_log_test.go`

**Interfaces:**
- Consumes: `GatewayContext.WorkspaceID`, `GatewayContext.APIKeyID`, `GatewayContext.APIKeyHash`.
- Produces: ClickHouse `access_logs` rows with `workspace_id`, `api_key_id`, `api_key_hash`.

- [ ] **Step 1: Write failing access log item test**

Add a test to `pkg/filters/outbound/access_log_test.go`:

```go
func TestAccessLogFilterBuildsPortalIdentityFields(t *testing.T) {
	f := NewAccessLogFilter(zap.NewNop(), nil, nil, nil, nil)
	gctx := &core.GatewayContext{
		StartTime:   time.Now().Add(-50 * time.Millisecond),
		APIKey:      "tl_live_abcdefghijklmnopqrstuvwxyz123456",
		APIKeyID:    "ak_1",
		APIKeyHash:  "hash_1",
		WorkspaceID: "wsp_1",
		UserID:      "usr_1",
		Model:       "gpt-4o",
	}

	item := f.buildAccessLogItem(gctx)

	if item.WorkspaceID != "wsp_1" || item.APIKeyID != "ak_1" || item.APIKeyHash != "hash_1" {
		t.Fatalf("portal identity = %+v", item)
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./pkg/filters/outbound -run TestAccessLogFilterBuildsPortalIdentityFields
```

Expected: FAIL because helper and fields do not exist.

- [ ] **Step 3: Extract access log item builder and fields**

In `pkg/filters/outbound/access_log.go`, add fields to `AccessLogItem`:

```go
WorkspaceID string `json:"workspace_id"`
APIKeyID    string `json:"api_key_id"`
APIKeyHash  string `json:"api_key_hash"`
```

Extract the item construction from `OnResponse` into:

```go
func (f *AccessLogFilter) buildAccessLogItem(gctx *core.GatewayContext) AccessLogItem {
	// existing construction logic
	item.WorkspaceID = gctx.WorkspaceID
	item.APIKeyID = gctx.APIKeyID
	item.APIKeyHash = gctx.APIKeyHash
	return item
}
```

Then `OnResponse` uses:

```go
item := f.buildAccessLogItem(gctx)
```

- [ ] **Step 4: Update ClickHouse DDL and batch append**

In `internal/repository/clickhouse.go`, add columns after `api_key String`:

```sql
workspace_id LowCardinality(String),
api_key_id LowCardinality(String),
api_key_hash String,
```

In `writeBatchToClickHouse`, append the new fields immediately after `item.APIKey`:

```go
item.WorkspaceID,
item.APIKeyID,
item.APIKeyHash,
```

- [ ] **Step 5: Run access log tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./pkg/filters/outbound
```

Expected: PASS.

- [ ] **Step 6: Run gateway verification**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./...
go build ./...
```

Expected: PASS.

---

### Task 3: Portal ClickHouse Configuration And Reader

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/go.mod`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/go.sum`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/config/config.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/config/config_test.go`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/usage/clickhouse.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/usage/clickhouse_test.go`

**Interfaces:**
- Produces: `usage.Reader` with `Summary(ctx, workspaceID, day)` and `RecentLogs(ctx, workspaceID, limit)`.
- Produces: `config.ClickHouseConfig`.

- [ ] **Step 1: Write failing config test**

Append to `backend/internal/config/config_test.go`:

```go
func TestLoadClickHouseUsageConfig(t *testing.T) {
	t.Setenv("PORTAL_USAGE_CLICKHOUSE_ENABLED", "true")
	t.Setenv("PORTAL_CLICKHOUSE_ADDR", " ch1:9000, ch2:9000 ")
	t.Setenv("PORTAL_CLICKHOUSE_DATABASE", " portal_usage ")
	t.Setenv("PORTAL_CLICKHOUSE_USERNAME", " user ")
	t.Setenv("PORTAL_CLICKHOUSE_PASSWORD", " secret ")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if !got.ClickHouse.Enabled {
		t.Fatalf("ClickHouse should be enabled")
	}
	if len(got.ClickHouse.Addr) != 2 || got.ClickHouse.Addr[0] != "ch1:9000" || got.ClickHouse.Addr[1] != "ch2:9000" {
		t.Fatalf("ClickHouse addr = %#v", got.ClickHouse.Addr)
	}
	if got.ClickHouse.Database != "portal_usage" || got.ClickHouse.Username != "user" || got.ClickHouse.Password != "secret" {
		t.Fatalf("ClickHouse config = %+v", got.ClickHouse)
	}
}
```

- [ ] **Step 2: Run config test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/config -run TestLoadClickHouseUsageConfig
```

Expected: FAIL because `Config.ClickHouse` does not exist.

- [ ] **Step 3: Implement config**

In `backend/internal/config/config.go`, add:

```go
type ClickHouseConfig struct {
	Enabled  bool
	Addr     []string
	Database string
	Username string
	Password string
}
```

Add `ClickHouse ClickHouseConfig` to `Config`.

Add:

```go
func loadClickHouseConfig() ClickHouseConfig {
	enabled := normalizeEnv(os.Getenv("PORTAL_USAGE_CLICKHOUSE_ENABLED")) == "true"
	addrRaw := strings.TrimSpace(os.Getenv("PORTAL_CLICKHOUSE_ADDR"))
	var addr []string
	for _, part := range strings.Split(addrRaw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			addr = append(addr, part)
		}
	}
	return ClickHouseConfig{
		Enabled:  enabled,
		Addr:     addr,
		Database: strings.TrimSpace(envOrDefault("PORTAL_CLICKHOUSE_DATABASE", "tokenlive_gateway")),
		Username: strings.TrimSpace(envOrDefault("PORTAL_CLICKHOUSE_USERNAME", "default")),
		Password: strings.TrimSpace(os.Getenv("PORTAL_CLICKHOUSE_PASSWORD")),
	}
}
```

Call it from `Load()`.

- [ ] **Step 4: Run config tests to verify GREEN**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/config
```

Expected: PASS.

- [ ] **Step 5: Add ClickHouse dependency**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go get github.com/ClickHouse/clickhouse-go/v2@v2.46.0
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 6: Create usage reader types**

Create `backend/internal/usage/clickhouse.go`:

```go
package usage

import (
	"context"
	"time"
)

type Summary struct {
	WorkspaceID             string
	RequestCount            int64
	SuccessCount            int64
	ErrorCount              int64
	InputTokens             int64
	OutputTokens            int64
	CachedTokens            int64
	CacheCreationTokens     int64
	CostCNY                 string
	AvgLatencyMs            int64
	AvgTTFTMs               int64
	Models                  []ModelSummary
}

type ModelSummary struct {
	Model        string
	RequestCount int64
	SuccessCount int64
	ErrorCount   int64
	InputTokens  int64
	OutputTokens int64
	CostCNY      string
}

type RequestLog struct {
	RequestID           string
	Time                time.Time
	Model               string
	APIKeyID            string
	APIKeyDisplay       string
	StatusCode          int16
	LatencyMs           int64
	TTFTMs              int64
	InputTokens         int64
	OutputTokens        int64
	CachedTokens        int64
	CacheCreationTokens int64
	CostCNY             string
	ErrorMessage        string
}

type Reader interface {
	Available() bool
	Summary(ctx context.Context, workspaceID string, day time.Time) (Summary, error)
	RecentLogs(ctx context.Context, workspaceID string, limit int) ([]RequestLog, error)
}

type DisabledReader struct{}

func (DisabledReader) Available() bool { return false }
func (DisabledReader) Summary(context.Context, string, time.Time) (Summary, error) {
	return Summary{}, nil
}
func (DisabledReader) RecentLogs(context.Context, string, int) ([]RequestLog, error) {
	return nil, nil
}
```

Add the concrete ClickHouse reader after service tests define exact expectations.

- [ ] **Step 7: Run usage package tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/usage
```

Expected: PASS after adding package tests or no tests yet.

---

### Task 4: Portal Usage Service And API

**Files:**
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/api/usage.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/api/console.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/api/console_test.go`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/usage/service.go`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/internal/usage/service_test.go`

**Interfaces:**
- Consumes: `usage.Reader`.
- Produces: `GET /api/usage/summary`, `GET /api/request-logs?limit=50`.

- [ ] **Step 1: Write failing service test for disabled reader**

Create `backend/internal/usage/service_test.go`:

```go
package usage

import (
	"context"
	"testing"
	"time"
)

func TestServiceSummaryUnavailableWhenReaderDisabled(t *testing.T) {
	svc := NewService(DisabledReader{}, func() time.Time {
		return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	})

	got, err := svc.Summary(context.Background(), "wsp_1")
	if err != nil {
		t.Fatalf("Summary() err = %v", err)
	}
	if got.Available || got.Today != nil || len(got.Models) != 0 {
		t.Fatalf("Summary() = %+v, want unavailable empty response", got)
	}
}
```

- [ ] **Step 2: Run service test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/usage -run TestServiceSummaryUnavailableWhenReaderDisabled
```

Expected: FAIL because `NewService` does not exist.

- [ ] **Step 3: Implement usage service response types**

Create `backend/internal/usage/service.go`:

```go
package usage

import (
	"context"
	"time"
)

type SummaryResponse struct {
	DataSource  string          `json:"data_source"`
	Available   bool            `json:"available"`
	WorkspaceID string          `json:"workspace_id"`
	Today       *TodaySummary   `json:"today"`
	Models      []ModelResponse `json:"models"`
}

type TodaySummary struct {
	RequestCount        int64  `json:"request_count"`
	SuccessCount        int64  `json:"success_count"`
	ErrorCount          int64  `json:"error_count"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CachedTokens        int64  `json:"cached_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CostCNY             string `json:"cost_cny"`
	AvgLatencyMs        int64  `json:"avg_latency_ms"`
	AvgTTFTMs           int64  `json:"avg_ttft_ms"`
}

type ModelResponse struct {
	Model        string `json:"model"`
	RequestCount int64  `json:"request_count"`
	SuccessCount int64  `json:"success_count"`
	ErrorCount   int64  `json:"error_count"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostCNY      string `json:"cost_cny"`
}

type RequestLogsResponse struct {
	Logs []RequestLogResponse `json:"logs"`
}

type RequestLogResponse struct {
	RequestID           string    `json:"request_id"`
	Time                time.Time `json:"time"`
	Model               string    `json:"model"`
	APIKeyID            string    `json:"api_key_id"`
	APIKeyDisplay       string    `json:"api_key_display"`
	StatusCode          int16     `json:"status_code"`
	LatencyMs           int64     `json:"latency_ms"`
	TTFTMs              int64     `json:"ttft_ms"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CachedTokens        int64     `json:"cached_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CostCNY             string    `json:"cost_cny"`
	ErrorMessage        string    `json:"error_message"`
}

type Service struct {
	reader Reader
	now    func() time.Time
}

func NewService(reader Reader, now func() time.Time) *Service {
	if reader == nil {
		reader = DisabledReader{}
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{reader: reader, now: now}
}

func (s *Service) Summary(ctx context.Context, workspaceID string) (SummaryResponse, error) {
	resp := SummaryResponse{DataSource: "clickhouse", Available: s.reader.Available(), WorkspaceID: workspaceID, Models: []ModelResponse{}}
	if !s.reader.Available() {
		return resp, nil
	}
	summary, err := s.reader.Summary(ctx, workspaceID, s.now())
	if err != nil {
		resp.Available = false
		return resp, nil
	}
	resp.Today = &TodaySummary{
		RequestCount: summary.RequestCount,
		SuccessCount: summary.SuccessCount,
		ErrorCount: summary.ErrorCount,
		InputTokens: summary.InputTokens,
		OutputTokens: summary.OutputTokens,
		CachedTokens: summary.CachedTokens,
		CacheCreationTokens: summary.CacheCreationTokens,
		CostCNY: summary.CostCNY,
		AvgLatencyMs: summary.AvgLatencyMs,
		AvgTTFTMs: summary.AvgTTFTMs,
	}
	for _, m := range summary.Models {
		resp.Models = append(resp.Models, ModelResponse{
			Model:        m.Model,
			RequestCount: m.RequestCount,
			SuccessCount: m.SuccessCount,
			ErrorCount:   m.ErrorCount,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			CostCNY:      m.CostCNY,
		})
	}
	return resp, nil
}

func (s *Service) RecentLogs(ctx context.Context, workspaceID string, limit int) (RequestLogsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if !s.reader.Available() {
		return RequestLogsResponse{Logs: []RequestLogResponse{}}, nil
	}
	logs, err := s.reader.RecentLogs(ctx, workspaceID, limit)
	if err != nil {
		return RequestLogsResponse{Logs: []RequestLogResponse{}}, nil
	}
	resp := RequestLogsResponse{Logs: make([]RequestLogResponse, 0, len(logs))}
	for _, log := range logs {
		resp.Logs = append(resp.Logs, RequestLogResponse{
			RequestID:           log.RequestID,
			Time:                log.Time,
			Model:               log.Model,
			APIKeyID:            log.APIKeyID,
			APIKeyDisplay:       log.APIKeyDisplay,
			StatusCode:          log.StatusCode,
			LatencyMs:           log.LatencyMs,
			TTFTMs:              log.TTFTMs,
			InputTokens:         log.InputTokens,
			OutputTokens:        log.OutputTokens,
			CachedTokens:        log.CachedTokens,
			CacheCreationTokens: log.CacheCreationTokens,
			CostCNY:             log.CostCNY,
			ErrorMessage:        log.ErrorMessage,
		})
	}
	return resp, nil
}
```

- [ ] **Step 4: Run service tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/usage
```

Expected: PASS.

- [ ] **Step 5: Write failing handler route test**

In `backend/internal/api/console_test.go`, add:

```go
func TestConsoleUsageSummaryRequiresSession(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, &fakeConsoleService{}, &fakeAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/api/usage/summary", nil)
	rec := httptest.NewRecorder()
	RequestID(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

Also add an authenticated happy-path fake after the route is added.

- [ ] **Step 6: Run handler test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/api -run TestConsoleUsageSummaryRequiresSession
```

Expected: FAIL with 404 because route does not exist.

- [ ] **Step 7: Add usage methods to `ConsoleService` and handler**

In `backend/internal/api/console_service.go`, add:

```go
UsageSummary(ctx context.Context, user CurrentUser) (usage.SummaryResponse, error)
RequestLogs(ctx context.Context, user CurrentUser, limit int) (usage.RequestLogsResponse, error)
```

Wire the concrete service to resolve current Workspace and call `usage.Service`.

In `backend/internal/api/console.go`, register:

```go
mux.HandleFunc("GET /api/usage/summary", handler.UsageSummary)
mux.HandleFunc("GET /api/request-logs", handler.RequestLogs)
```

Handler pattern:

```go
func (h ConsoleHandler) UsageSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.UsageSummary(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 8: Run API tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./internal/api
```

Expected: PASS.

---

### Task 5: Portal API Wiring

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/cmd/portal-api/main.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/backend/cmd/portal-api/main_test.go`

**Interfaces:**
- Consumes: `config.ClickHouseConfig`.
- Produces: Console service with a usage service backed by ClickHouse or disabled reader.

- [ ] **Step 1: Write failing route wiring test**

In `backend/cmd/portal-api/main_test.go`, add `/api/usage/summary` and `/api/request-logs` to the registered routes assertion list in `TestRegisterDatabaseBackedRoutesRegistersPublicAuthAndConsoleRoutes`.

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./cmd/portal-api -run TestRegisterDatabaseBackedRoutesRegistersPublicAuthAndConsoleRoutes
```

Expected: FAIL if routes are not registered by the fake console route seam.

- [ ] **Step 3: Extend seams and wiring**

In `backend/cmd/portal-api/main.go`, create a seam:

```go
newPortalUsageReader = usage.NewClickHouseReader
```

In `registerDatabaseBackedRoutes`, construct usage reader from `cfg.ClickHouse`, pass `usage.NewService(reader, time.Now)` to the console service constructor, or register usage routes separately with an explicit usage handler.

Keep disabled behavior:

```go
reader := usage.Reader(usage.DisabledReader{})
if cfg.ClickHouse.Enabled {
	reader, closeUsage, err := newPortalUsageReader(cfg.ClickHouse)
	// on error, log and keep disabled reader unless config error should block startup
}
```

- [ ] **Step 4: Run portal API wiring tests**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./cmd/portal-api
```

Expected: PASS.

---

### Task 6: Portal Frontend Usage Page

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/web/src/types/api.ts`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/web/src/lib/api.ts`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/web/src/lib/usage-display.js`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/web/tests/usage-display.test.mjs`
- Create: `/Users/chenzhiguo/Projects/tokenlive-portal/web/src/app/console/usage/page.tsx`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-portal/web/src/app/console/layout.tsx`

**Interfaces:**
- Consumes: `GET /api/usage/summary`, `GET /api/request-logs`.
- Produces: `/console/usage` route.

- [ ] **Step 1: Write failing frontend helper test**

Create `web/tests/usage-display.test.mjs`:

```js
import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  formatCostCNY,
  formatLatency,
  formatSuccessRate,
  formatTokens,
} from "../src/lib/usage-display.js";

describe("usage display helpers", () => {
  it("formats usage numbers", () => {
    assert.equal(formatTokens(1234567), "1,234,567");
    assert.equal(formatCostCNY("1.234567"), "¥1.234567");
    assert.equal(formatLatency(321), "321 ms");
    assert.equal(formatSuccessRate(40, 42), "95.2%");
  });

  it("formats empty usage values", () => {
    assert.equal(formatCostCNY(""), "-");
    assert.equal(formatLatency(0), "-");
    assert.equal(formatSuccessRate(0, 0), "-");
  });
});
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/web
npm run test:unit -- usage-display.test.mjs
```

Expected: FAIL because helper file does not exist.

- [ ] **Step 3: Implement helper**

Create `web/src/lib/usage-display.js`:

```js
export function formatTokens(value) {
  return Number(value || 0).toLocaleString();
}

export function formatCostCNY(value) {
  const text = String(value ?? "").trim();
  return text ? `¥${text}` : "-";
}

export function formatLatency(value) {
  const n = Number(value || 0);
  return n > 0 ? `${n.toLocaleString()} ms` : "-";
}

export function formatSuccessRate(success, total) {
  const s = Number(success || 0);
  const t = Number(total || 0);
  if (t <= 0) return "-";
  return `${((s / t) * 100).toFixed(1)}%`;
}
```

- [ ] **Step 4: Add TypeScript API types and clients**

In `web/src/types/api.ts`, add:

```ts
export interface UsageSummaryResponse {
  data_source: "clickhouse";
  available: boolean;
  workspace_id: string;
  today: UsageTodaySummary | null;
  models: UsageModelSummary[];
}
```

Add `UsageTodaySummary`, `UsageModelSummary`, `RequestLogsResponse`, and `RequestLogResponse` matching the spec.

In `web/src/lib/api.ts`, add:

```ts
export async function fetchUsageSummary(): Promise<UsageSummaryResponse> {
  return request<UsageSummaryResponse>("/api/usage/summary");
}

export async function fetchRequestLogs(limit = 50): Promise<RequestLogsResponse> {
  return request<RequestLogsResponse>(`/api/request-logs?limit=${limit}`);
}
```

- [ ] **Step 5: Create `/console/usage` page**

Create `web/src/app/console/usage/page.tsx` using the same client-side loading pattern as dashboard and billing:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { fetchRequestLogs, fetchUsageSummary } from "@/lib/api";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";
import { formatCostCNY, formatLatency, formatSuccessRate, formatTokens } from "@/lib/usage-display";
import type { RequestLogsResponse, UsageSummaryResponse } from "@/types/api";
```

Render:

- unavailable state if `summary?.available === false`
- cards for today metrics
- model table
- recent requests table

- [ ] **Step 6: Add nav item**

In `web/src/app/console/layout.tsx`, import `Activity` from `lucide-react` and add:

```ts
{ href: "/console/usage", label: "Usage", icon: Activity },
```

between API Keys and Billing.

- [ ] **Step 7: Run frontend tests and build**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/web
npm run test:unit
npm run lint
npm run build
```

Expected: PASS, with `/console/usage` in the Next route list.

---

### Task 7: End-To-End Verification

**Files:**
- No new files.

**Interfaces:**
- Verifies Gateway and Portal build/test health.

- [ ] **Step 1: Run Gateway verification**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-gateway
go test ./...
go build ./...
```

Expected: PASS.

- [ ] **Step 2: Run Portal backend verification**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/backend
go test ./...
go build ./...
```

Expected: PASS.

- [ ] **Step 3: Run Portal frontend verification**

Run:

```bash
cd /Users/chenzhiguo/Projects/tokenlive-portal/web
npm run test:unit
npm run lint
npm run build
```

Expected: PASS.

- [ ] **Step 4: Check git status in both repositories**

Run:

```bash
git -C /Users/chenzhiguo/Projects/tokenlive-gateway status --short --branch
git -C /Users/chenzhiguo/Projects/tokenlive-portal status --short --branch
```

Expected: only intentional files are modified.

---

## Self-Review Notes

- Spec coverage: Gateway ClickHouse fields, Portal ClickHouse read client, Usage Summary API, Request Logs API, frontend page, security/privacy, unavailable state, and deferred ledger work are covered.
- No Portal Redis usage counters are introduced.
- Request Logs expose redacted `api_key_display` and `api_key_id`, not `api_key_hash`.
- Implementation touches `tokenlive-gateway`, which is outside the current Portal workspace root and may require filesystem approval during execution.
