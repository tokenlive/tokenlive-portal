package api

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type redisRuntimeClient interface {
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type redisAPIKeyRuntimeSyncer struct {
	client redisRuntimeClient
}

func NewRedisAPIKeyRuntimeSyncer(client redisRuntimeClient) APIKeyRuntimeSyncer {
	if client == nil {
		return NewNoopAPIKeyRuntimeSyncer()
	}
	return redisAPIKeyRuntimeSyncer{client: client}
}

func (s redisAPIKeyRuntimeSyncer) UpsertAPIKey(ctx context.Context, record APIKeyRuntimeRecord) error {
	return s.client.HSet(ctx, redisKeyAPIKeyHash(record.KeyHash),
		"source", "portal",
		"key_id", record.KeyID,
		"user_id", record.UserID,
		"workspace_id", record.WorkspaceID,
		"scope_type", record.ScopeType,
		"scope_code", record.ScopeCode,
		"tenant", record.Tenant,
		"user_tenant", record.UserTenant,
		"status", record.Status,
		"quota", record.Quota,
		"expires_at", record.ExpiresAt,
	).Err()
}

func (s redisAPIKeyRuntimeSyncer) DeleteAPIKey(ctx context.Context, keyHash string) error {
	return s.client.Del(ctx, redisKeyAPIKeyHash(keyHash)).Err()
}

func redisKeyAPIKeyHash(keyHash string) string {
	return "aigw:apikey_hash:" + keyHash
}
