package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRedisAPIKeyRuntimeSyncerUpsertWritesGatewayHashKey(t *testing.T) {
	t.Parallel()

	client := &fakeRedisRuntimeClient{}
	syncer := NewRedisAPIKeyRuntimeSyncer(client)

	err := syncer.UpsertAPIKey(context.Background(), APIKeyRuntimeRecord{
		KeyHash:     "hash_1",
		KeyID:       "ak_1",
		UserID:      "usr_1",
		WorkspaceID: "wsp_1",
		ScopeType:   "tenant",
		ScopeCode:   "tenant_a",
		Tenant:      "tenant_a",
		UserTenant:  "tenant_a",
		Status:      1,
		Quota:       -1,
		ExpiresAt:   1784647200,
	})
	if err != nil {
		t.Fatalf("UpsertAPIKey() err = %v", err)
	}

	if client.hsetKey != "aigw:apikey_hash:hash_1" {
		t.Fatalf("hset key = %q, want hash runtime key", client.hsetKey)
	}
	wantValues := []any{
		"source", "portal",
		"key_id", "ak_1",
		"user_id", "usr_1",
		"workspace_id", "wsp_1",
		"scope_type", "tenant",
		"scope_code", "tenant_a",
		"tenant", "tenant_a",
		"user_tenant", "tenant_a",
		"status", 1,
		"quota", int64(-1),
		"expires_at", int64(1784647200),
	}
	if !reflect.DeepEqual(client.hsetValues, wantValues) {
		t.Fatalf("hset values = %#v, want %#v", client.hsetValues, wantValues)
	}
}

func TestRedisAPIKeyRuntimeSyncerDeleteRemovesGatewayHashKey(t *testing.T) {
	t.Parallel()

	client := &fakeRedisRuntimeClient{}
	syncer := NewRedisAPIKeyRuntimeSyncer(client)

	err := syncer.DeleteAPIKey(context.Background(), "hash_1")
	if err != nil {
		t.Fatalf("DeleteAPIKey() err = %v", err)
	}

	if len(client.delKeys) != 1 || client.delKeys[0] != "aigw:apikey_hash:hash_1" {
		t.Fatalf("del keys = %#v, want hash runtime key", client.delKeys)
	}
}

type fakeRedisRuntimeClient struct {
	hsetKey    string
	hsetValues []any
	delKeys    []string
}

func (f *fakeRedisRuntimeClient) HSet(_ context.Context, key string, values ...any) *redis.IntCmd {
	f.hsetKey = key
	f.hsetValues = append([]any(nil), values...)
	return redis.NewIntResult(1, nil)
}

func (f *fakeRedisRuntimeClient) Del(_ context.Context, keys ...string) *redis.IntCmd {
	f.delKeys = append([]string(nil), keys...)
	return redis.NewIntResult(int64(len(keys)), nil)
}
