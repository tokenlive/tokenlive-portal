package config

import (
	"strings"
	"testing"
)

func TestLoadAuthPepperDefaultDevelopment(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_AUTH_PEPPER", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.AuthPepper != "dev-auth-pepper" {
		t.Fatalf("AuthPepper = %q, want %q", got.AuthPepper, "dev-auth-pepper")
	}
}

func TestLoadAuthPepperEmptyInProductionWhenUnset(t *testing.T) {
	t.Setenv("PORTAL_ENV", " Production ")
	t.Setenv("PORTAL_AUTH_PEPPER", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.Env != "production" {
		t.Fatalf("Env = %q, want production", got.Env)
	}
	if got.AuthPepper != "" {
		t.Fatalf("AuthPepper = %q, want empty string", got.AuthPepper)
	}
}

func TestLoadAuthPepperRespectsExplicitValue(t *testing.T) {
	t.Setenv("PORTAL_ENV", "production")
	t.Setenv("PORTAL_AUTH_PEPPER", "custom-pepper")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.AuthPepper != "custom-pepper" {
		t.Fatalf("AuthPepper = %q, want %q", got.AuthPepper, "custom-pepper")
	}
}

func TestLoadTrialCreditDefaults(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_AUTH_PEPPER", "")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.TrialCredit.AmountMicroCNY != 10_000_000 {
		t.Fatalf("TrialCredit.AmountMicroCNY = %d, want 10000000", got.TrialCredit.AmountMicroCNY)
	}
	if got.TrialCredit.TTLDays != 7 {
		t.Fatalf("TrialCredit.TTLDays = %d, want 7", got.TrialCredit.TTLDays)
	}
}

func TestLoadTrialCreditEnvOverrides(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "2500000")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "14")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.TrialCredit.AmountMicroCNY != 2_500_000 {
		t.Fatalf("TrialCredit.AmountMicroCNY = %d, want 2500000", got.TrialCredit.AmountMicroCNY)
	}
	if got.TrialCredit.TTLDays != 14 {
		t.Fatalf("TrialCredit.TTLDays = %d, want 14", got.TrialCredit.TTLDays)
	}
}

func TestLoadTrialCreditRejectsNegativeAmount(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "-1")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_MICRO_CNY") {
		t.Fatalf("Load() err = %v, want trial amount error", err)
	}
}

func TestLoadTrialCreditRejectsNonIntegerAmount(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "10.5")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_MICRO_CNY") {
		t.Fatalf("Load() err = %v, want trial amount error", err)
	}
}

func TestLoadTrialCreditRejectsInvalidTTL(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_TTL_DAYS") {
		t.Fatalf("Load() err = %v, want trial ttl error", err)
	}
}

func TestLoadTrialCreditRejectsNegativeTTL(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "-1")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_TTL_DAYS") {
		t.Fatalf("Load() err = %v, want trial ttl error", err)
	}
}

func TestLoadTrialCreditRejectsNonIntegerTTL(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "seven")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_TTL_DAYS") {
		t.Fatalf("Load() err = %v, want trial ttl error", err)
	}
}

func TestLoadOAuthConfigs(t *testing.T) {
	t.Setenv("PORTAL_GOOGLE_CLIENT_ID", " google-client ")
	t.Setenv("PORTAL_GOOGLE_CLIENT_SECRET", " google-secret ")
	t.Setenv("PORTAL_GOOGLE_REDIRECT_URL", " https://portal.example.com/google ")
	t.Setenv("PORTAL_GITHUB_CLIENT_ID", " github-client ")
	t.Setenv("PORTAL_GITHUB_CLIENT_SECRET", " github-secret ")
	t.Setenv("PORTAL_GITHUB_REDIRECT_URL", " https://portal.example.com/github ")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if !got.GoogleOAuth.Enabled() {
		t.Fatalf("GoogleOAuth should be enabled")
	}
	if got.GoogleOAuth.ClientID != "google-client" || got.GoogleOAuth.ClientSecret != "google-secret" || got.GoogleOAuth.RedirectURL != "https://portal.example.com/google" {
		t.Fatalf("GoogleOAuth = %+v", got.GoogleOAuth)
	}
	if !got.GitHubOAuth.Enabled() {
		t.Fatalf("GitHubOAuth should be enabled")
	}
	if got.GitHubOAuth.ClientID != "github-client" || got.GitHubOAuth.ClientSecret != "github-secret" || got.GitHubOAuth.RedirectURL != "https://portal.example.com/github" {
		t.Fatalf("GitHubOAuth = %+v", got.GitHubOAuth)
	}
}

func TestLoadGatewayRedisConfig(t *testing.T) {
	t.Setenv("PORTAL_GATEWAY_REDIS_ADDR", " redis.example.com:6379 ")
	t.Setenv("PORTAL_GATEWAY_REDIS_PASSWORD", " secret ")
	t.Setenv("PORTAL_GATEWAY_REDIS_DB", "5")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.GatewayRedis.Addr != "redis.example.com:6379" || got.GatewayRedis.Password != "secret" || got.GatewayRedis.DB != 5 {
		t.Fatalf("GatewayRedis = %+v", got.GatewayRedis)
	}
	if !got.GatewayRedis.Enabled() {
		t.Fatalf("GatewayRedis should be enabled")
	}
}

func TestLoadGatewayRedisRejectsInvalidDB(t *testing.T) {
	t.Setenv("PORTAL_GATEWAY_REDIS_ADDR", "redis.example.com:6379")
	t.Setenv("PORTAL_GATEWAY_REDIS_DB", "bad")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_GATEWAY_REDIS_DB") {
		t.Fatalf("Load() err = %v, want gateway redis db error", err)
	}
}

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

func TestLoadClickHouseUsageConfigDisabledByDefault(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if got.ClickHouse.Enabled {
		t.Fatalf("ClickHouse should be disabled by default")
	}
	if got.ClickHouse.Database != "tokenlive_gateway" {
		t.Fatalf("ClickHouse database = %q, want tokenlive_gateway", got.ClickHouse.Database)
	}
	if got.ClickHouse.Username != "default" {
		t.Fatalf("ClickHouse username = %q, want default", got.ClickHouse.Username)
	}
}
