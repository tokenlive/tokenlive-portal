# Gateway Portal API Key Hash Lookup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `tokenlive-gateway` authenticate Portal-owned API keys through `aigw:apikey_hash:{key_hash}` and align with the unified Admin/Gateway hash runtime contract.

**Architecture:** Add a Portal-compatible HMAC helper and Redis key helper in Gateway. `RedisGatewayProvider` receives the configured API key pepper, computes the hash, and reads the runtime hash key. Gateway wiring reads the pepper from config. Redis plaintext API key lookup is intentionally not part of the final runtime contract.

**Tech Stack:** Go 1.x, Redis, miniredis tests, `crypto/hmac`, `crypto/sha256`, existing `GatewayProvider` interface.

## Global Constraints

- Do not write or read `aigw:apikey:{plaintext_api_key}` in the final runtime path.
- Do not store or log Portal API key plaintext beyond the already-extracted request key.
- Portal hash lookup uses the same HMAC-SHA256 algorithm as `tokenlive-portal/backend/internal/security.HashAPIKey`.
- First-release Portal runtime keys set `quota = -1`; do not reinterpret Portal `micro_cny` limits as Gateway token quota.
- Gateway rejects Redis-backed API key lookup when `llm.api_key_pepper` is empty.
- Do not commit automatically; the user controls commits.

---

## File Structure

Modify in `/Users/chenzhiguo/Projects/tokenlive-gateway`:

- `pkg/store/keys.go`: add a helper for `aigw:apikey_hash:{key_hash}`.
- `pkg/config/api_key_hash.go`: new focused helper for HMAC-SHA256 Portal API key hash computation.
- `pkg/config/redis_provider.go`: add API key pepper and hash-only lookup.
- `pkg/config/redis_provider_test.go`: new miniredis tests for Portal hash lookup and legacy plaintext rejection.
- `cmd/server/wire/engine.go`: pass the configured API key pepper when constructing the Redis provider.
- `cmd/server/wire/wire_gen.go`: mirror the generated wiring change if this repository does not regenerate Wire during normal local work.
- `config/local.yml.example`, `config/prod.yml`, `config/local.yml`: add `llm.api_key_pepper` using environment expansion.

No `tokenlive-portal` or `tokenlive-admin` code changes are part of this plan.

---

### Task 1: Add Gateway Helpers For Portal Hash Runtime Keys

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/store/keys.go`
- Create: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/api_key_hash.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/api_key_hash_test.go`

**Interfaces:**
- Produces: `store.RedisKeyApiKeyHash(keyHash string) string`
- Produces: `config.HashAPIKey(apiKey string, pepper string) string`

- [ ] **Step 1: Write failing tests for hash and Redis key helpers**

Create `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/api_key_hash_test.go`:

```go
package config

import "testing"

func TestHashAPIKeyMatchesPortalHMACSHA256(t *testing.T) {
	got := HashAPIKey("tl_live_example", "pepper")
	want := "06bfbed9282f1dcb96bd25c7bef96d9b49de0be5f3777b44f4f71cfcca8821b1"
	if got != want {
		t.Fatalf("HashAPIKey() = %q, want %q", got, want)
	}
}

func TestHashAPIKeyEmptyPepperStillDeterministic(t *testing.T) {
	got := HashAPIKey("tl_live_example", "")
	want := "569f98d4f3a9dd99afb439d169cf4ee2bcc98ffd60181c4a6476627dff65010a"
	if got != want {
		t.Fatalf("HashAPIKey() = %q, want %q", got, want)
	}
}
```

Append this test to a new `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/store/keys_test.go` if the file does not already exist:

```go
package store

import "testing"

func TestRedisKeyApiKeyHash(t *testing.T) {
	got := RedisKeyApiKeyHash("hash123")
	want := "aigw:apikey_hash:hash123"
	if got != want {
		t.Fatalf("RedisKeyApiKeyHash() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run from `/Users/chenzhiguo/Projects/tokenlive-gateway`:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./pkg/config ./pkg/store
```

Expected: FAIL because `HashAPIKey` and `RedisKeyApiKeyHash` are undefined.

- [ ] **Step 3: Implement helpers**

Add to `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/store/keys.go`:

```go
func RedisKeyApiKeyHash(keyHash string) string {
	return "aigw:apikey_hash:" + keyHash
}
```

Create `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/api_key_hash.go`:

```go
package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func HashAPIKey(apiKey string, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(apiKey))
	return hex.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 4: Run tests and verify they pass**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./pkg/config ./pkg/store
```

Expected: PASS.

---

### Task 2: Make RedisGatewayProvider Prefer Portal Hash Lookup

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider.go`
- Test: `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider_test.go`

**Interfaces:**
- Consumes: `store.RedisKeyApiKeyHash(keyHash string) string`
- Consumes: `config.HashAPIKey(apiKey string, pepper string) string`
- Produces: `config.NewRedisGatewayProviderWithAPIKeyPepper(rdb *redis.Client, apiKeyPepper string) *RedisGatewayProvider`

- [ ] **Step 1: Write failing Redis provider tests**

Create `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider_test.go`:

```go
package config

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/tokenlive/tokenlive-gateway/pkg/store"
)

func newRedisProviderTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client, mr
}

func TestRedisGatewayProviderGetApiKeyPrefersPortalHashLookup(t *testing.T) {
	client, mr := newRedisProviderTestClient(t)
	ctx := context.Background()
	apiKey := "tl_live_hash_lookup"
	pepper := "api-key-pepper"
	keyHash := HashAPIKey(apiKey, pepper)

	mr.HSet(store.RedisKeyApiKeyHash(keyHash),
		"key_id", "ak_1",
		"user_id", "usr_1",
		"workspace_id", "wsp_1",
		"tenant", "tenant_a",
		"user_tenant", "tenant_a",
		"status", "1",
		"quota", "-1",
		"expires_at", "0",
	)

	provider := NewRedisGatewayProviderWithAPIKeyPepper(client, pepper)
	got, err := provider.GetApiKey(ctx, apiKey)
	if err != nil {
		t.Fatalf("GetApiKey() err = %v", err)
	}
	if got.UserID != "usr_1" || got.WorkspaceID != "wsp_1" || got.Tenant != "tenant_a" || got.Quota != -1 {
		t.Fatalf("GetApiKey() = %+v, want portal hash fields", got)
	}
}

func TestRedisGatewayProviderGetApiKeyIgnoresLegacyPlaintext(t *testing.T) {
	client, mr := newRedisProviderTestClient(t)
	ctx := context.Background()
	apiKey := "sk-legacy-key"

	mr.HSet(store.RedisKeyApiKey(apiKey),
		"user_id", "legacy_user",
		"user_tenant", "legacy_tenant",
		"status", "1",
		"quota", "500",
		"expires_at", "0",
	)

	provider := NewRedisGatewayProviderWithAPIKeyPepper(client, "api-key-pepper")
	if _, err := provider.GetApiKey(ctx, apiKey); err == nil {
		t.Fatalf("GetApiKey() err = nil, want missing hash key error")
	}
}

func TestRedisGatewayProviderGetApiKeyRequiresPepper(t *testing.T) {
	client, mr := newRedisProviderTestClient(t)
	ctx := context.Background()
	apiKey := "tl_live_no_pepper"
	keyHash := HashAPIKey(apiKey, "api-key-pepper")

	mr.HSet(store.RedisKeyApiKeyHash(keyHash),
		"user_id", "usr_1",
		"workspace_id", "wsp_1",
		"status", "1",
		"quota", "-1",
		"expires_at", "0",
	)

	provider := NewRedisGatewayProvider(client)
	_, err := provider.GetApiKey(ctx, apiKey)
	if err == nil {
		t.Fatalf("GetApiKey() err = nil, want missing api_key_pepper error")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./pkg/config
```

Expected: FAIL because `NewRedisGatewayProviderWithAPIKeyPepper` is undefined and hash lookup is not implemented.

- [ ] **Step 3: Update RedisGatewayProvider struct and constructors**

Modify `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider.go`:

```go
type RedisGatewayProvider struct {
	rdb                *redis.Client
	apiKeyPepper string
}

func NewRedisGatewayProvider(rdb *redis.Client) *RedisGatewayProvider {
	return NewRedisGatewayProviderWithAPIKeyPepper(rdb, "")
}

func NewRedisGatewayProviderWithAPIKeyPepper(rdb *redis.Client, apiKeyPepper string) *RedisGatewayProvider {
	return &RedisGatewayProvider{
		rdb:                rdb,
		apiKeyPepper: apiKeyPepper,
	}
}
```

- [ ] **Step 4: Extract Redis hash parsing helper**

Add in `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider.go` near `GetApiKey`:

```go
func parseRedisApiKeyItem(apiKey string, fields map[string]string) (*HTTPApiKeyItem, bool) {
	if len(fields) == 0 || (fields["user_id"] == "" && fields["tenant"] == "" && fields["workspace_id"] == "") {
		return nil, false
	}

	status, _ := strconv.Atoi(fields["status"])
	quota, _ := strconv.ParseInt(fields["quota"], 10, 64)
	expiresAt, _ := strconv.ParseInt(fields["expires_at"], 10, 64)

	return &HTTPApiKeyItem{
		APIKey:      apiKey,
		UserID:      fields["user_id"],
		Tenant:      fields["tenant"],
		WorkspaceID: fields["workspace_id"],
		UserTenant:  fields["user_tenant"],
		Status:      status,
		Quota:       quota,
		ExpiresAt:   expiresAt,
	}, true
}
```

- [ ] **Step 5: Implement hash-only lookup**

Replace `GetApiKey` and add a shared resolver in `/Users/chenzhiguo/Projects/tokenlive-gateway/pkg/config/redis_provider.go`:

```go
func (p *RedisGatewayProvider) GetApiKey(ctx context.Context, apiKey string) (*HTTPApiKeyItem, error) {
	item, _, err := p.getApiKeyWithRedisKey(ctx, apiKey)
	return item, err
}

func (p *RedisGatewayProvider) getApiKeyWithRedisKey(ctx context.Context, apiKey string) (*HTTPApiKeyItem, string, error) {
	if p.apiKeyPepper == "" {
		return nil, "", fmt.Errorf("llm.api_key_pepper is required for redis api key lookup")
	}

	keyHash := HashAPIKey(apiKey, p.apiKeyPepper)
	redisKey := store.RedisKeyApiKeyHash(keyHash)
	fields, err := p.rdb.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, "", err
	}
	if item, ok := parseRedisApiKeyItem(apiKey, fields); ok {
		return item, redisKey, nil
	}

	return nil, "", fmt.Errorf("api key not found in redis")
}
```

Keep existing imports for `context`, `fmt`, `strconv`, and `store`.

- [ ] **Step 6: Run tests and verify they pass**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./pkg/config
```

Expected: PASS.

---

### Task 3: Wire Portal Pepper From Gateway Config

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/cmd/server/wire/engine.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/cmd/server/wire/wire_gen.go`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/config/local.yml.example`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/config/local.yml`
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/config/prod.yml`

**Interfaces:**
- Consumes: `config.NewRedisGatewayProviderWithAPIKeyPepper(rdb, pepper)`
- Produces config key: `llm.api_key_pepper`

- [ ] **Step 1: Add config key to YAML files**

Under each existing `llm:` block in the three config files, add:

```yaml
  api_key_pepper: ${GATEWAY_API_KEY_PEPPER:}
```

Place it next to `enable_auth` so operators can find it with other auth settings.

- [ ] **Step 2: Update provider construction in engine wiring**

In `/Users/chenzhiguo/Projects/tokenlive-gateway/cmd/server/wire/engine.go`, replace each Redis provider construction:

```go
return config.NewRedisGatewayProvider(rdb), nil
```

with:

```go
apiKeyPepper := v.GetString("llm.api_key_pepper")
return config.NewRedisGatewayProviderWithAPIKeyPepper(rdb, apiKeyPepper), nil
```

Apply this to the Redis provider path used when Gateway reads runtime config from Redis. If the file contains more than one Redis provider construction, update each one consistently.

- [ ] **Step 3: Mirror generated wiring**

If `/Users/chenzhiguo/Projects/tokenlive-gateway/cmd/server/wire/wire_gen.go` directly constructs `config.NewRedisGatewayProvider(rdb)`, make the same replacement there:

```go
apiKeyPepper := v.GetString("llm.api_key_pepper")
gatewayProvider := config.NewRedisGatewayProviderWithAPIKeyPepper(rdb, apiKeyPepper)
```

If `wire_gen.go` only calls functions that were changed in `engine.go`, leave it untouched.

- [ ] **Step 4: Run focused tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./cmd/server/wire ./pkg/config
```

Expected: PASS.

---

### Task 4: Verify ApiKeyService Behavior For Portal Hash Keys

**Files:**
- Modify: `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/service/apikey_test.go`
- No production code expected unless the tests reveal a service-layer issue.

**Interfaces:**
- Consumes: `config.NewRedisGatewayProviderWithAPIKeyPepper`
- Consumes: `config.HashAPIKey`
- Consumes: `store.RedisKeyApiKeyHash`

- [ ] **Step 1: Add miniredis-style service test in package config if avoiding real Redis**

Prefer not to extend `internal/service/apikey_test.go` because it currently uses real Redis. Instead, create `/Users/chenzhiguo/Projects/tokenlive-gateway/internal/service/apikey_hash_test.go` with miniredis:

```go
package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/tokenlive/tokenlive-gateway/pkg/config"
	"github.com/tokenlive/tokenlive-gateway/pkg/log"
	"github.com/tokenlive/tokenlive-gateway/pkg/store"
	"go.uber.org/zap"
)

func TestApiKeyServiceValidatePortalHashKey(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	apiKey := "tl_live_service_hash"
	pepper := "api-key-pepper"
	keyHash := config.HashAPIKey(apiKey, pepper)
	mr.HSet(store.RedisKeyApiKeyHash(keyHash),
		"user_id", "usr_1",
		"workspace_id", "wsp_1",
		"tenant", "tenant_a",
		"user_tenant", "tenant_a",
		"status", "1",
		"quota", "-1",
		"expires_at", "0",
	)

	logger := &log.Logger{Logger: zap.NewNop()}
	svc := NewApiKeyService(config.NewRedisGatewayProviderWithAPIKeyPepper(client, pepper), logger)
	info, err := svc.ValidateKey(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("ValidateKey() err = %v", err)
	}
	if info.UserID != "usr_1" || info.WorkspaceID != "wsp_1" || info.Tenant != "tenant_a" || info.UserTenant != "tenant_a" {
		t.Fatalf("ValidateKey() = %+v, want portal runtime identity", info)
	}
}
```

- [ ] **Step 2: Run service test and verify it passes**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/service -run TestApiKeyServiceValidatePortalHashKey
```

Expected: PASS.

- [ ] **Step 3: Run package tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./pkg/config ./pkg/store ./internal/service
```

Expected: PASS. Existing real Redis tests may skip when Redis is unavailable.

---

### Task 5: Full Gateway Verification

**Files:**
- No source changes expected.

**Interfaces:**
- Verifies all earlier tasks.

- [ ] **Step 1: Run full Gateway tests**

Run from `/Users/chenzhiguo/Projects/tokenlive-gateway`:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS, with any existing real-Redis tests skipped if Redis is unavailable.

- [ ] **Step 2: Run formatting**

Run:

```bash
gofmt -w pkg/store/keys.go pkg/config/api_key_hash.go pkg/config/api_key_hash_test.go pkg/config/redis_provider.go pkg/config/redis_provider_test.go internal/service/apikey_hash_test.go cmd/server/wire/engine.go cmd/server/wire/wire_gen.go
```

Expected: command exits 0 and only formats touched Go files.

- [ ] **Step 3: Check diff hygiene**

Run from `/Users/chenzhiguo/Projects/tokenlive-gateway`:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 4: Manual smoke shape**

With Gateway configured with:

```bash
GATEWAY_API_KEY_PEPPER=<same value as Portal API key pepper>
```

and Redis seeded with:

```text
aigw:apikey_hash:{HMAC_SHA256(portal_key, pepper)}
  user_id=usr_1
  workspace_id=wsp_1
  tenant=tenant_a
  user_tenant=tenant_a
  status=1
  quota=-1
  expires_at=0
```

Expected behavior:

- Calling Gateway with `Authorization: Bearer <portal_key>` authenticates.
- Gateway request context includes `X-Workspace-ID: wsp_1`.
- Changing `status` to `2` rejects the key after local cache expiry or after `ApiKeyService.PurgeCache()`.
