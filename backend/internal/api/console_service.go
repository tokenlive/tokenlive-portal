package api

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/money"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

const maxAPIKeyNameLength = 160

var (
	ErrConsoleWorkspaceNotFound       = errors.New("console workspace not found")
	ErrConsolePermissionDenied        = errors.New("console permission denied")
	ErrConsoleAPIKeyNotFound          = errors.New("console api key not found")
	ErrConsoleAPIKeyInvalidState      = errors.New("console api key invalid state")
	ErrConsoleAPIKeyInvalidName       = errors.New("console api key invalid name")
	ErrConsoleAPIKeyInvalidLimit      = errors.New("console api key invalid limit")
	ErrConsoleAPIKeyInvalidExpiration = errors.New("console api key invalid expiration")
)

type ConsoleService interface {
	Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error)
	CurrentWorkspace(ctx context.Context, user CurrentUser) (CurrentWorkspaceResponse, error)
	ListAPIKeys(ctx context.Context, user CurrentUser) (ListAPIKeysResponse, error)
	CreateAPIKey(ctx context.Context, user CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error)
	EnableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
	DisableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
	RevokeAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
}

type ActivationStepStatus string

const (
	ActivationStepCompleted ActivationStepStatus = "completed"
	ActivationStepPending   ActivationStepStatus = "pending"
)

type MoneyAmount struct {
	MicroCNY *int64  `json:"micro_cny,omitempty"`
	CNY      *string `json:"cny,omitempty"`
}

type WorkspaceBalanceResponse struct {
	AvailableMicroCNY int64  `json:"available_micro_cny"`
	FrozenMicroCNY    int64  `json:"frozen_micro_cny"`
	AvailableCNY      string `json:"available_cny"`
	FrozenCNY         string `json:"frozen_cny"`
}

type CurrentWorkspaceResponse struct {
	Workspace WorkspaceResponse `json:"workspace"`
}

type ConsoleOverviewResponse struct {
	Workspace  WorkspaceResponse          `json:"workspace"`
	Activation ActivationOverviewResponse `json:"activation"`
}

type ActivationOverviewResponse struct {
	TrialCreditGranted bool                     `json:"trial_credit_granted"`
	TrialExpiresAt     *time.Time               `json:"trial_expires_at"`
	APIKeyCreated      bool                     `json:"api_key_created"`
	FirstCallMade      bool                     `json:"first_call_made"`
	Steps              []ActivationStepResponse `json:"steps"`
}

type ActivationStepResponse struct {
	Key    string               `json:"key"`
	Label  string               `json:"label"`
	Status ActivationStepStatus `json:"status"`
}

type WorkspaceResponse struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Slug           string                   `json:"slug"`
	Role           domain.MemberRole        `json:"role"`
	Status         domain.WorkspaceStatus   `json:"status"`
	TrialGrantedAt *time.Time               `json:"trial_granted_at"`
	Balance        WorkspaceBalanceResponse `json:"balance"`
}

type APIKeyResponse struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	KeyPrefix            string              `json:"key_prefix"`
	SecretLast4          string              `json:"secret_last4"`
	Status               domain.APIKeyStatus `json:"status"`
	ExpiresAt            *time.Time          `json:"expires_at"`
	DailyLimitMicroCNY   *int64              `json:"daily_limit_micro_cny"`
	DailyLimitCNY        *string             `json:"daily_limit_cny"`
	MonthlyLimitMicroCNY *int64              `json:"monthly_limit_micro_cny"`
	MonthlyLimitCNY      *string             `json:"monthly_limit_cny"`
	LastUsedAt           *time.Time          `json:"last_used_at"`
	TotalSpendMicroCNY   int64               `json:"total_spend_micro_cny"`
	TotalSpendCNY        string              `json:"total_spend_cny"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

type ListAPIKeysResponse struct {
	Data []APIKeyResponse `json:"data"`
}

type CreateAPIKeyRequest struct {
	Name                 string     `json:"name"`
	DailyLimitMicroCNY   *int64     `json:"daily_limit_micro_cny"`
	MonthlyLimitMicroCNY *int64     `json:"monthly_limit_micro_cny"`
	ExpiresAt            *time.Time `json:"expires_at"`
}

type CreateAPIKeyResponse struct {
	APIKey APIKeyResponse `json:"api_key"`
	Secret string         `json:"secret"`
}

type consoleStore interface {
	ResolveCurrentWorkspace(ctx context.Context, userID string) (repository.CurrentWorkspaceResult, error)
	ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error)
	CreateAPIKeyWithAudit(ctx context.Context, input repository.CreateAPIKeyWithAuditInput) (repository.CreateAPIKeyResult, error)
	UpdateAPIKeyStatusWithAudit(ctx context.Context, input repository.UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error)
}

type consoleService struct {
	store       consoleStore
	authPepper  string
	trialCredit config.TrialCreditConfig
	nowFunc     func() time.Time
}

func newConsoleService(store consoleStore, authPepper string, trialCredit config.TrialCreditConfig) (*consoleService, error) {
	if store == nil {
		return nil, errors.New("console store is required")
	}
	if strings.TrimSpace(authPepper) == "" {
		return nil, errors.New("auth pepper must not be empty")
	}
	if trialCredit.AmountMicroCNY < 0 {
		return nil, errors.New("trial credit amount must be greater than or equal to zero")
	}
	if trialCredit.TTLDays <= 0 {
		return nil, errors.New("trial credit ttl must be greater than zero")
	}
	return &consoleService{
		store:       store,
		authPepper:  authPepper,
		trialCredit: trialCredit,
		nowFunc:     func() time.Time { return time.Now().UTC() },
	}, nil
}

func NewConsoleService(repos *repository.Repositories, authPepper string, trialCredit config.TrialCreditConfig) (ConsoleService, error) {
	if repos == nil {
		return nil, errors.New("console repositories are required")
	}
	return newConsoleService(repos, authPepper, trialCredit)
}

func (s *consoleService) Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return ConsoleOverviewResponse{}, err
	}

	keys, err := s.store.ListAPIKeysByWorkspace(ctx, current.Workspace.ID)
	if err != nil {
		return ConsoleOverviewResponse{}, mapConsoleRepositoryError(err)
	}

	trialGranted := current.Workspace.TrialGrantedAt != nil
	var trialExpiresAt *time.Time
	if current.Workspace.TrialGrantedAt != nil {
		expires := current.Workspace.TrialGrantedAt.AddDate(0, 0, s.trialCredit.TTLDays)
		trialExpiresAt = &expires
	}
	apiKeyCreated := len(keys) > 0
	firstCallMade := false

	return ConsoleOverviewResponse{
		Workspace: workspaceResponseFromRepository(current),
		Activation: ActivationOverviewResponse{
			TrialCreditGranted: trialGranted,
			TrialExpiresAt:     trialExpiresAt,
			APIKeyCreated:      apiKeyCreated,
			FirstCallMade:      firstCallMade,
			Steps:              activationSteps(trialGranted, apiKeyCreated, firstCallMade),
		},
	}, nil
}

func (s *consoleService) CurrentWorkspace(ctx context.Context, user CurrentUser) (CurrentWorkspaceResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return CurrentWorkspaceResponse{}, err
	}
	return CurrentWorkspaceResponse{Workspace: workspaceResponseFromRepository(current)}, nil
}

func (s *consoleService) ListAPIKeys(ctx context.Context, user CurrentUser) (ListAPIKeysResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return ListAPIKeysResponse{}, err
	}
	keys, err := s.store.ListAPIKeysByWorkspace(ctx, current.Workspace.ID)
	if err != nil {
		return ListAPIKeysResponse{}, mapConsoleRepositoryError(err)
	}
	resp := ListAPIKeysResponse{Data: make([]APIKeyResponse, 0, len(keys))}
	for _, key := range keys {
		resp.Data = append(resp.Data, apiKeyResponseFromDomain(key))
	}
	return resp, nil
}

func (s *consoleService) CreateAPIKey(ctx context.Context, user CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	createInput, err := s.validateCreateAPIKeyInput(input, s.nowFunc())
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	createInput.WorkspaceID = current.Workspace.ID
	createInput.CreatedByUserID = user.ID
	createInput.Pepper = s.authPepper

	created, err := s.store.CreateAPIKeyWithAudit(ctx, repository.CreateAPIKeyWithAuditInput{
		CreateAPIKeyInput: createInput,
		ActorUserID:       user.ID,
	})
	if err != nil {
		return CreateAPIKeyResponse{}, mapConsoleRepositoryError(err)
	}
	return CreateAPIKeyResponse{
		APIKey: apiKeyResponseFromDomain(created.APIKey),
		Secret: created.Secret,
	}, nil
}

func (s *consoleService) EnableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusEnabled)
}

func (s *consoleService) DisableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusDisabled)
}

func (s *consoleService) RevokeAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusRevoked)
}

func (s *consoleService) updateAPIKeyStatus(ctx context.Context, user CurrentUser, apiKeyID string, status domain.APIKeyStatus) (APIKeyResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return APIKeyResponse{}, err
	}
	apiKeyID = strings.TrimSpace(apiKeyID)
	if apiKeyID == "" {
		return APIKeyResponse{}, ErrConsoleAPIKeyNotFound
	}
	updated, err := s.store.UpdateAPIKeyStatusWithAudit(ctx, repository.UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: current.Workspace.ID,
		APIKeyID:    apiKeyID,
		Status:      status,
		ActorUserID: user.ID,
	})
	if err != nil {
		return APIKeyResponse{}, mapConsoleRepositoryError(err)
	}
	return apiKeyResponseFromDomain(updated), nil
}

func (s *consoleService) resolveWorkspace(ctx context.Context, user CurrentUser) (repository.CurrentWorkspaceResult, error) {
	userID := strings.TrimSpace(user.ID)
	if userID == "" {
		return repository.CurrentWorkspaceResult{}, ErrAuthUnauthorized
	}
	if !user.TermsAccepted {
		return repository.CurrentWorkspaceResult{}, ErrAuthTermsAlreadyAccepted
	}
	current, err := s.store.ResolveCurrentWorkspace(ctx, userID)
	if err != nil {
		return repository.CurrentWorkspaceResult{}, mapConsoleRepositoryError(err)
	}
	return current, nil
}

func (s *consoleService) resolveManageableWorkspace(ctx context.Context, user CurrentUser) (repository.CurrentWorkspaceResult, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return repository.CurrentWorkspaceResult{}, err
	}
	if !canManageAPIKeys(current.Role) {
		return repository.CurrentWorkspaceResult{}, ErrConsolePermissionDenied
	}
	return current, nil
}

func mapConsoleRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrWorkspaceNotFound):
		return ErrConsoleWorkspaceNotFound
	case errors.Is(err, repository.ErrAPIKeyNotFound):
		return ErrConsoleAPIKeyNotFound
	case errors.Is(err, repository.ErrAPIKeyInvalidState):
		return ErrConsoleAPIKeyInvalidState
	default:
		return err
	}
}

func mapConsoleError(err error) AppError {
	switch {
	case errors.Is(err, ErrConsoleWorkspaceNotFound):
		return ErrWorkspaceNotFound
	case errors.Is(err, ErrConsolePermissionDenied):
		return ErrPermissionDenied
	case errors.Is(err, ErrConsoleAPIKeyNotFound):
		return ErrAPIKeyNotFound
	case errors.Is(err, ErrConsoleAPIKeyInvalidState):
		return ErrAPIKeyInvalidState
	case errors.Is(err, ErrConsoleAPIKeyInvalidName):
		return ErrAPIKeyInvalidName
	case errors.Is(err, ErrConsoleAPIKeyInvalidLimit):
		return ErrAPIKeyInvalidLimit
	case errors.Is(err, ErrConsoleAPIKeyInvalidExpiration):
		return ErrAPIKeyInvalidExpiration
	case errors.Is(err, ErrAuthUnauthorized):
		return ErrUnauthorized
	case errors.Is(err, ErrAuthTermsAlreadyAccepted):
		return ErrTermsRequired
	default:
		return ErrInternalError
	}
}

func workspaceResponseFromRepository(current repository.CurrentWorkspaceResult) WorkspaceResponse {
	return WorkspaceResponse{
		ID:             current.Workspace.ID,
		Name:           current.Workspace.Name,
		Slug:           current.Workspace.Slug,
		Role:           current.Role,
		Status:         current.Workspace.Status,
		TrialGrantedAt: current.Workspace.TrialGrantedAt,
		Balance: WorkspaceBalanceResponse{
			AvailableMicroCNY: current.Balance.AvailableMicroCNY,
			FrozenMicroCNY:    current.Balance.FrozenMicroCNY,
			AvailableCNY:      money.MicroCNY(current.Balance.AvailableMicroCNY).FormatCNY(),
			FrozenCNY:         money.MicroCNY(current.Balance.FrozenMicroCNY).FormatCNY(),
		},
	}
}

func activationSteps(trialGranted bool, apiKeyCreated bool, firstCallMade bool) []ActivationStepResponse {
	return []ActivationStepResponse{
		{Key: "trial_credit", Label: "Receive trial credit", Status: activationStatus(trialGranted)},
		{Key: "api_key", Label: "Create API key", Status: activationStatus(apiKeyCreated)},
		{Key: "first_call", Label: "Make first API call", Status: activationStatus(firstCallMade)},
	}
}

func activationStatus(done bool) ActivationStepStatus {
	if done {
		return ActivationStepCompleted
	}
	return ActivationStepPending
}

func apiKeyResponseFromDomain(key domain.APIKey) APIKeyResponse {
	daily := moneyAmountFromPointer(key.DailyLimitMicroCNY)
	monthly := moneyAmountFromPointer(key.MonthlyLimitMicroCNY)
	return APIKeyResponse{
		ID:                   key.ID,
		Name:                 key.Name,
		KeyPrefix:            key.KeyPrefix,
		SecretLast4:          key.SecretLast4,
		Status:               key.Status,
		ExpiresAt:            key.ExpiresAt,
		DailyLimitMicroCNY:   daily.MicroCNY,
		DailyLimitCNY:        daily.CNY,
		MonthlyLimitMicroCNY: monthly.MicroCNY,
		MonthlyLimitCNY:      monthly.CNY,
		LastUsedAt:           key.LastUsedAt,
		TotalSpendMicroCNY:   key.TotalSpendMicroCNY,
		TotalSpendCNY:        money.MicroCNY(key.TotalSpendMicroCNY).FormatCNY(),
		CreatedAt:            key.CreatedAt,
		UpdatedAt:            key.UpdatedAt,
	}
}

func moneyAmountFromPointer(value *int64) MoneyAmount {
	if value == nil {
		return MoneyAmount{}
	}
	formatted := money.MicroCNY(*value).FormatCNY()
	return MoneyAmount{
		MicroCNY: value,
		CNY:      &formatted,
	}
}

func canManageAPIKeys(role domain.MemberRole) bool {
	return role == domain.MemberRoleOwner || role == domain.MemberRoleDeveloper
}

func (s *consoleService) validateCreateAPIKeyInput(input CreateAPIKeyRequest, now time.Time) (repository.CreateAPIKeyInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > maxAPIKeyNameLength {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidName
	}
	if input.DailyLimitMicroCNY != nil && *input.DailyLimitMicroCNY < 0 {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidLimit
	}
	if input.MonthlyLimitMicroCNY != nil && *input.MonthlyLimitMicroCNY < 0 {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidLimit
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidExpiration
	}

	return repository.CreateAPIKeyInput{
		Name:                 name,
		DailyLimitMicroCNY:   input.DailyLimitMicroCNY,
		MonthlyLimitMicroCNY: input.MonthlyLimitMicroCNY,
		ExpiresAt:            input.ExpiresAt,
	}, nil
}
