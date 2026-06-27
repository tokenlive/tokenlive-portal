package domain

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserStatus string
type WorkspaceStatus string
type MemberRole string
type MemberStatus string
type InvitationStatus string
type APIKeyStatus string
type LedgerType string
type LedgerDirection string
type ModelCatalogStatus string
type ModelCatalogVisibility string
type ModelPriceStatus string
type SessionStatus string
type EmailCodeStatus string
type EmailCodePurpose string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusBlocked  UserStatus = "blocked"
	UserStatusDeleting UserStatus = "deleting"

	WorkspaceStatusActive   WorkspaceStatus = "active"
	WorkspaceStatusBlocked  WorkspaceStatus = "blocked"
	WorkspaceStatusDeleting WorkspaceStatus = "deleting"

	MemberRoleOwner     MemberRole = "owner"
	MemberRoleDeveloper MemberRole = "developer"
	MemberRoleBilling   MemberRole = "billing"

	MemberStatusActive  MemberStatus = "active"
	MemberStatusRemoved MemberStatus = "removed"

	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired"

	APIKeyStatusEnabled  APIKeyStatus = "enabled"
	APIKeyStatusDisabled APIKeyStatus = "disabled"
	APIKeyStatusRevoked  APIKeyStatus = "revoked"

	LedgerTypeRecharge    LedgerType = "recharge"
	LedgerTypeTrialGrant  LedgerType = "trial_grant"
	LedgerTypeConsumption LedgerType = "consumption"
	LedgerTypeRefund      LedgerType = "refund"
	LedgerTypeAdjustment  LedgerType = "adjustment"

	LedgerDirectionCredit LedgerDirection = "credit"
	LedgerDirectionDebit  LedgerDirection = "debit"

	ModelCatalogStatusAvailable ModelCatalogStatus = "available"
	ModelCatalogStatusPaused    ModelCatalogStatus = "paused"

	ModelCatalogVisibilityPublic  ModelCatalogVisibility = "public"
	ModelCatalogVisibilityPrivate ModelCatalogVisibility = "private"

	ModelPriceStatusActive   ModelPriceStatus = "active"
	ModelPriceStatusInactive ModelPriceStatus = "inactive"

	SessionStatusActive  SessionStatus = "active"
	SessionStatusRevoked SessionStatus = "revoked"

	EmailCodeStatusPending  EmailCodeStatus = "pending"
	EmailCodeStatusConsumed EmailCodeStatus = "consumed"
	EmailCodeStatusBlocked  EmailCodeStatus = "blocked"

	EmailCodePurposeLogin EmailCodePurpose = "login"
)

type User struct {
	ID              string  `gorm:"primaryKey;size:32"`
	DisplayName     string  `gorm:"size:120;not null;default:''"`
	PrimaryEmail    *string `gorm:"size:320;uniqueIndex:uk_users_primary_email"`
	EmailVerifiedAt *time.Time
	TermsAcceptedAt *time.Time
	AvatarURL       string         `gorm:"size:1024;not null;default:''"`
	Status          UserStatus     `gorm:"size:32;not null;index"`
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

type AccountIdentity struct {
	ID              string     `gorm:"primaryKey;size:32"`
	UserID          string     `gorm:"size:32;not null;uniqueIndex:uk_account_identities_user_provider,priority:1"`
	Provider        string     `gorm:"size:32;not null;uniqueIndex:uk_account_identities_provider_subject,priority:1;uniqueIndex:uk_account_identities_user_provider,priority:2"`
	ProviderSubject string     `gorm:"size:191;not null;uniqueIndex:uk_account_identities_provider_subject,priority:2"`
	Email           string     `gorm:"size:320;not null;default:''"`
	EmailVerified   bool       `gorm:"not null;default:false"`
	DisplayName     string     `gorm:"size:120;not null;default:''"`
	AvatarURL       string     `gorm:"size:1024;not null;default:''"`
	LinkedAt        *time.Time
	CreatedAt       time.Time  `gorm:"not null"`
	UpdatedAt       time.Time  `gorm:"not null"`
}

type Workspace struct {
	ID              string          `gorm:"primaryKey;size:32"`
	Name            string          `gorm:"size:160;not null"`
	Slug            string          `gorm:"size:160;not null;uniqueIndex:uk_workspaces_slug"`
	OwnerUserID     string          `gorm:"size:32;not null;index"`
	TenantCode      *string         `gorm:"size:64;default:null"`
	Status          WorkspaceStatus `gorm:"size:32;not null"`
	TrialGrantedAt  *time.Time
	CreatedByUserID string         `gorm:"size:32;not null;index"`
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

type WorkspaceMember struct {
	WorkspaceID string       `gorm:"primaryKey;size:32"`
	UserID      string       `gorm:"primaryKey;size:32;index"`
	Role        MemberRole   `gorm:"size:32;not null"`
	Status      MemberStatus `gorm:"size:32;not null"`
	JoinedAt    *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

type UserSession struct {
	ID         string        `gorm:"primaryKey;size:32"`
	UserID     string        `gorm:"size:32;not null;index:idx_user_sessions_user_status,priority:1"`
	TokenHash  string        `gorm:"size:128;not null;uniqueIndex:uk_user_sessions_token_hash"`
	Status     SessionStatus `gorm:"size:32;not null;index:idx_user_sessions_user_status,priority:2"`
	IP         string        `gorm:"size:64;not null;default:''"`
	UserAgent  string        `gorm:"size:512;not null;default:''"`
	ExpiresAt  time.Time     `gorm:"not null;index"`
	LastSeenAt *time.Time
	CreatedAt  time.Time `gorm:"not null"`
	RevokedAt  *time.Time
}

type EmailVerificationCode struct {
	ID           string           `gorm:"primaryKey;size:32"`
	Email        string           `gorm:"size:320;not null;index:idx_email_codes_lookup,priority:1"`
	Purpose      EmailCodePurpose `gorm:"size:32;not null;index:idx_email_codes_lookup,priority:2"`
	CodeHash     string           `gorm:"size:128;not null"`
	Status       EmailCodeStatus  `gorm:"size:32;not null;index:idx_email_codes_lookup,priority:3"`
	AttemptCount int              `gorm:"not null;default:0"`
	ExpiresAt    time.Time        `gorm:"not null"`
	ConsumedAt   *time.Time
	CreatedAt    time.Time `gorm:"not null;index:idx_email_codes_lookup,priority:4"`
}

type WorkspaceInvitation struct {
	ID               string           `gorm:"primaryKey;size:32"`
	WorkspaceID      string           `gorm:"size:32;not null;index:idx_workspace_invitations_workspace_email,priority:1"`
	Email            string           `gorm:"size:320;not null;index:idx_workspace_invitations_workspace_email,priority:2"`
	Role             MemberRole       `gorm:"size:32;not null"`
	TokenHash        string           `gorm:"size:128;not null;uniqueIndex:uk_workspace_invitations_token_hash"`
	Status           InvitationStatus `gorm:"size:32;not null;index"`
	InvitedByUserID  string           `gorm:"size:32;not null"`
	AcceptedByUserID *string          `gorm:"size:32"`
	ExpiresAt        time.Time        `gorm:"not null"`
	AcceptedAt       *time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

type APIKey struct {
	ID                   string       `gorm:"primaryKey;size:32"`
	WorkspaceID          string       `gorm:"size:32;not null;index"`
	Name                 string       `gorm:"size:160;not null"`
	KeyPrefix            string       `gorm:"size:32;not null"`
	SecretLast4          string       `gorm:"size:8;not null"`
	KeyHash              string       `gorm:"size:128;not null;uniqueIndex:uk_api_keys_key_hash"`
	Status               APIKeyStatus `gorm:"size:32;not null;index"`
	CreatedByUserID      string       `gorm:"size:32;not null;index"`
	ExpiresAt            *time.Time
	DailyLimitMicroCNY   *int64
	MonthlyLimitMicroCNY *int64
	LastUsedAt           *time.Time
	TotalSpendMicroCNY   int64     `gorm:"not null;default:0"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
	RevokedAt            *time.Time
}

type WorkspaceModelPermission struct {
	WorkspaceID     string    `gorm:"primaryKey;size:32"`
	ModelID         string    `gorm:"primaryKey;size:191;index"`
	Source          string    `gorm:"size:32;not null"`
	GrantedByUserID *string   `gorm:"size:32"`
	CreatedAt       time.Time `gorm:"not null"`
}

type APIKeyModelWhitelist struct {
	APIKeyID  string    `gorm:"primaryKey;size:32"`
	ModelID   string    `gorm:"primaryKey;size:191;index"`
	CreatedAt time.Time `gorm:"not null"`
}

type WorkspaceBalance struct {
	WorkspaceID       string    `gorm:"primaryKey;size:32"`
	AvailableMicroCNY int64     `gorm:"not null;default:0"`
	FrozenMicroCNY    int64     `gorm:"not null;default:0"`
	Version           int64     `gorm:"not null;default:1"`
	UpdatedAt         time.Time `gorm:"not null"`
}

type LedgerEntry struct {
	ID                       string          `gorm:"primaryKey;size:32"`
	WorkspaceID              string          `gorm:"size:32;not null;index:idx_ledger_entries_workspace_created,priority:1;uniqueIndex:uk_ledger_entries_workspace_idempotency,priority:1"`
	Type                     LedgerType      `gorm:"size:32;not null"`
	Direction                LedgerDirection `gorm:"size:16;not null"`
	AmountMicroCNY           int64           `gorm:"not null"`
	BalanceAfterMicroCNY     int64           `gorm:"not null"`
	Currency                 string          `gorm:"size:8;not null"`
	IdempotencyKey           string          `gorm:"size:191;not null;uniqueIndex:uk_ledger_entries_workspace_idempotency,priority:2"`
	RequestID                *string         `gorm:"size:191;uniqueIndex:uk_ledger_entries_request_id"`
	APIKeyID                 *string         `gorm:"size:32;index"`
	APIKeyNameSnapshot       string          `gorm:"size:160;not null;default:''"`
	ModelID                  string          `gorm:"size:191;not null;default:''"`
	ModelDisplayNameSnapshot string          `gorm:"size:255;not null;default:''"`
	PriceVersionID           string          `gorm:"size:32;not null;default:''"`
	UnitPriceSnapshot        datatypes.JSON
	Metadata                 datatypes.JSON
	CreatedAt                time.Time `gorm:"not null;index:idx_ledger_entries_workspace_created,priority:2"`
}

type AuditLog struct {
	ID           string  `gorm:"primaryKey;size:32"`
	WorkspaceID  *string `gorm:"size:32;index:idx_audit_logs_workspace_created,priority:1"`
	ActorUserID  *string `gorm:"size:32;index:idx_audit_logs_actor_created,priority:1"`
	Action       string  `gorm:"size:96;not null"`
	ResourceType string  `gorm:"size:64;not null;index:idx_audit_logs_resource,priority:1"`
	ResourceID   string  `gorm:"size:64;not null;index:idx_audit_logs_resource,priority:2"`
	BeforeData   datatypes.JSON
	AfterData    datatypes.JSON
	IP           string    `gorm:"size:64;not null;default:''"`
	UserAgent    string    `gorm:"size:512;not null;default:''"`
	CreatedAt    time.Time `gorm:"not null;index:idx_audit_logs_workspace_created,priority:2;index:idx_audit_logs_actor_created,priority:2"`
}

type ModelCatalog struct {
	ModelID          string                 `gorm:"primaryKey;size:191"`
	Slug             string                 `gorm:"size:191;not null;uniqueIndex:uk_model_catalogs_slug"`
	Status           ModelCatalogStatus     `gorm:"size:32;not null;index:idx_model_catalogs_public_list,priority:2"`
	Visibility       ModelCatalogVisibility `gorm:"size:32;not null;index:idx_model_catalogs_public_list,priority:1"`
	LogoURL          string                 `gorm:"size:1024;not null;default:''"`
	ContextLength    *int64
	KnowledgeCutoff  *time.Time `gorm:"type:date"`
	InputModalities  datatypes.JSON
	OutputModalities datatypes.JSON
	Capabilities     datatypes.JSON
	Featured         bool       `gorm:"not null;default:false;index:idx_model_catalogs_public_list,priority:3"`
	SortWeight       int64      `gorm:"not null;default:0;index:idx_model_catalogs_public_list,priority:4"`
	PublishedAt      *time.Time `gorm:"index:idx_model_catalogs_public_list,priority:5"`
	CreatedAt        time.Time  `gorm:"not null"`
	UpdatedAt        time.Time  `gorm:"not null"`
}

type ModelCatalogI18n struct {
	ModelID          string  `gorm:"primaryKey;size:191"`
	Locale           string  `gorm:"primaryKey;size:16"`
	DisplayName      string  `gorm:"size:255;not null"`
	ShortDescription string  `gorm:"size:512;not null;default:''"`
	LongDescription  *string `gorm:"type:text"`
	SEOTitle         string  `gorm:"size:255;not null;default:''"`
	SEODescription   string  `gorm:"size:512;not null;default:''"`
	Tags             datatypes.JSON
	UpdatedAt        time.Time `gorm:"not null"`
}

func (ModelCatalogI18n) TableName() string {
	return "model_catalog_i18n"
}

type ModelPriceVersion struct {
	ID                           string           `gorm:"primaryKey;size:32"`
	ModelID                      string           `gorm:"size:191;not null;uniqueIndex:uk_model_price_versions_model_effective,priority:1;index:idx_model_price_versions_current,priority:1"`
	Currency                     string           `gorm:"size:8;not null"`
	InputMicroCNYPer1MTokens     int64            `gorm:"column:input_micro_cny_per_1m_tokens;not null"`
	OutputMicroCNYPer1MTokens    int64            `gorm:"column:output_micro_cny_per_1m_tokens;not null"`
	CacheReadMicroCNYPer1MTokens *int64           `gorm:"column:cache_read_micro_cny_per_1m_tokens"`
	EffectiveFrom                time.Time        `gorm:"not null;uniqueIndex:uk_model_price_versions_model_effective,priority:2;index:idx_model_price_versions_current,priority:3"`
	EffectiveUntil               *time.Time       `gorm:"index:idx_model_price_versions_current,priority:4"`
	Status                       ModelPriceStatus `gorm:"size:32;not null;index:idx_model_price_versions_current,priority:2"`
	PublishedByUserID            *string          `gorm:"size:32"`
	PublishedAt                  time.Time        `gorm:"not null"`
}

type ModelServiceMetric struct {
	ModelID       string    `gorm:"primaryKey;size:191"`
	Window        string    `gorm:"primaryKey;size:16"`
	Availability  *float64  `gorm:"type:decimal(8,6)"`
	TTFTP50MS     *int64    `gorm:"column:ttft_p50_ms"`
	TTFTP95MS     *int64    `gorm:"column:ttft_p95_ms"`
	ResponseSpeed *float64  `gorm:"type:decimal(18,6)"`
	SuccessRate   *float64  `gorm:"type:decimal(8,6)"`
	SampleCount   int64     `gorm:"not null;default:0"`
	UpdatedAt     time.Time `gorm:"not null"`
}
