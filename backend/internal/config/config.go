package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultTrialCreditMicroCNY = int64(10_000_000)
	defaultTrialCreditTTLDays  = 7
)

type Config struct {
	Env              string
	HTTPAddr         string
	DatabaseDSN      string
	AuthPepper       string
	InternalAPIToken string
	TrialCredit      TrialCreditConfig
	GatewayRedis     GatewayRedisConfig
	GoogleOAuth      GoogleOAuthConfig
	GitHubOAuth      GitHubOAuthConfig
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (c GoogleOAuthConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (c GitHubOAuthConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

type TrialCreditConfig struct {
	AmountMicroCNY int64
	TTLDays        int
}

type GatewayRedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func (c GatewayRedisConfig) Enabled() bool {
	return c.Addr != ""
}

func Load() (Config, error) {
	env := normalizeEnv(envOrDefault("PORTAL_ENV", "development"))
	authPepper := os.Getenv("PORTAL_AUTH_PEPPER")
	if authPepper == "" && env != "production" {
		authPepper = "dev-auth-pepper"
	}

	internalAPIToken := os.Getenv("PORTAL_INTERNAL_API_TOKEN")
	if internalAPIToken == "" && env != "production" {
		internalAPIToken = "dev-internal-token"
	}

	trialCredit, err := loadTrialCreditConfig()
	if err != nil {
		return Config{}, err
	}
	gatewayRedis, err := loadGatewayRedisConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Env:              env,
		HTTPAddr:         envOrDefault("PORTAL_HTTP_ADDR", ":8080"),
		DatabaseDSN:      os.Getenv("PORTAL_DATABASE_DSN"),
		AuthPepper:       authPepper,
		InternalAPIToken: internalAPIToken,
		TrialCredit:      trialCredit,
		GatewayRedis:     gatewayRedis,
		GoogleOAuth:      loadGoogleOAuthConfig(),
		GitHubOAuth:      loadGitHubOAuthConfig(),
	}, nil
}

func loadGoogleOAuthConfig() GoogleOAuthConfig {
	return GoogleOAuthConfig{
		ClientID:     strings.TrimSpace(os.Getenv("PORTAL_GOOGLE_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("PORTAL_GOOGLE_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(os.Getenv("PORTAL_GOOGLE_REDIRECT_URL")),
	}
}

func loadGitHubOAuthConfig() GitHubOAuthConfig {
	return GitHubOAuthConfig{
		ClientID:     strings.TrimSpace(os.Getenv("PORTAL_GITHUB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("PORTAL_GITHUB_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(os.Getenv("PORTAL_GITHUB_REDIRECT_URL")),
	}
}

func loadTrialCreditConfig() (TrialCreditConfig, error) {
	amount, err := int64EnvOrDefault("PORTAL_TRIAL_CREDIT_MICRO_CNY", defaultTrialCreditMicroCNY)
	if err != nil {
		return TrialCreditConfig{}, err
	}
	if amount < 0 {
		return TrialCreditConfig{}, fmt.Errorf("PORTAL_TRIAL_CREDIT_MICRO_CNY must be greater than or equal to zero")
	}

	ttlDays, err := intEnvOrDefault("PORTAL_TRIAL_CREDIT_TTL_DAYS", defaultTrialCreditTTLDays)
	if err != nil {
		return TrialCreditConfig{}, err
	}
	if ttlDays <= 0 {
		return TrialCreditConfig{}, fmt.Errorf("PORTAL_TRIAL_CREDIT_TTL_DAYS must be greater than zero")
	}

	return TrialCreditConfig{
		AmountMicroCNY: amount,
		TTLDays:        ttlDays,
	}, nil
}

func loadGatewayRedisConfig() (GatewayRedisConfig, error) {
	db, err := intEnvOrDefault("PORTAL_GATEWAY_REDIS_DB", 0)
	if err != nil {
		return GatewayRedisConfig{}, err
	}
	if db < 0 {
		return GatewayRedisConfig{}, fmt.Errorf("PORTAL_GATEWAY_REDIS_DB must be greater than or equal to zero")
	}
	return GatewayRedisConfig{
		Addr:     strings.TrimSpace(os.Getenv("PORTAL_GATEWAY_REDIS_ADDR")),
		Password: strings.TrimSpace(os.Getenv("PORTAL_GATEWAY_REDIS_PASSWORD")),
		DB:       db,
	}, nil
}

func int64EnvOrDefault(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func intEnvOrDefault(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func normalizeEnv(env string) string {
	return strings.ToLower(strings.TrimSpace(env))
}
