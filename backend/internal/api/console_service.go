package api

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/money"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"github.com/tokenlive/tokenlive-portal/backend/internal/usage"
)

const (
	maxAPIKeyNameLength      = 160
	maxRechargePaymentLength = 64
	maxRechargeContactLength = 320
	maxRechargeNoteLength    = 2000
	rechargeRequestListLimit = 20
	ledgerEntryListLimit     = 20
)

var (
	ErrConsoleWorkspaceNotFound       = errors.New("console workspace not found")
	ErrConsolePermissionDenied        = errors.New("console permission denied")
	ErrConsoleAPIKeyNotFound          = errors.New("console api key not found")
	ErrConsoleAPIKeyInvalidState      = errors.New("console api key invalid state")
	ErrConsoleAPIKeyInvalidName       = errors.New("console api key invalid name")
	ErrConsoleAPIKeyInvalidLimit      = errors.New("console api key invalid limit")
	ErrConsoleAPIKeyInvalidExpiration = errors.New("console api key invalid expiration")
	ErrConsoleRechargeInvalidAmount   = errors.New("console recharge invalid amount")
	ErrConsoleRechargeInvalidRequest  = errors.New("console recharge invalid request")
)

type ConsoleService interface {
	Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error)
	CurrentWorkspace(ctx context.Context, user CurrentUser) (CurrentWorkspaceResponse, error)
	BillingOverview(ctx context.Context, user CurrentUser) (BillingOverviewResponse, error)
	UsageSummary(ctx context.Context, user CurrentUser) (usage.SummaryResponse, error)
	RequestLogs(ctx context.Context, user CurrentUser, limit int) (usage.RequestLogsResponse, error)
	CreateRechargeRequest(ctx context.Context, user CurrentUser, input CreateRechargeRequestRequest) (CreateRechargeRequestResponse, error)
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
	RuntimeActivated   bool                     `json:"runtime_activated"`
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

type BillingOverviewResponse struct {
	Workspace        WorkspaceResponse         `json:"workspace"`
	RechargeRequests []RechargeRequestResponse `json:"recharge_requests"`
	LedgerEntries    []LedgerEntryResponse     `json:"ledger_entries"`
}

type LedgerEntryResponse struct {
	ID                       string                 `json:"id"`
	Type                     domain.LedgerType      `json:"type"`
	Direction                domain.LedgerDirection `json:"direction"`
	AmountMicroCNY           int64                  `json:"amount_micro_cny"`
	AmountCNY                string                 `json:"amount_cny"`
	BalanceAfterMicroCNY     int64                  `json:"balance_after_micro_cny"`
	BalanceAfterCNY          string                 `json:"balance_after_cny"`
	Currency                 string                 `json:"currency"`
	APIKeyID                 *string                `json:"api_key_id,omitempty"`
	APIKeyNameSnapshot       string                 `json:"api_key_name_snapshot"`
	ModelID                  string                 `json:"model_id"`
	ModelDisplayNameSnapshot string                 `json:"model_display_name_snapshot"`
	CreatedAt                time.Time              `json:"created_at"`
}

type RechargeRequestResponse struct {
	ID                string                       `json:"id"`
	RequestedByUserID string                       `json:"requested_by_user_id"`
	AmountMicroCNY    int64                        `json:"amount_micro_cny"`
	AmountCNY         string                       `json:"amount_cny"`
	Currency          string                       `json:"currency"`
	Status            domain.RechargeRequestStatus `json:"status"`
	PaymentMethod     string                       `json:"payment_method"`
	Contact           string                       `json:"contact"`
	Note              string                       `json:"note"`
	AdminNote         string                       `json:"admin_note"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
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

type CreateRechargeRequestRequest struct {
	AmountMicroCNY int64  `json:"amount_micro_cny"`
	PaymentMethod  string `json:"payment_method"`
	Contact        string `json:"contact"`
	Note           string `json:"note"`
}

type CreateRechargeRequestResponse struct {
	RechargeRequest RechargeRequestResponse `json:"recharge_request"`
}

type consoleStore interface {
	ResolveCurrentWorkspace(ctx context.Context, userID string) (repository.CurrentWorkspaceResult, error)
	ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error)
	CreateAPIKeyWithAudit(ctx context.Context, input repository.CreateAPIKeyWithAuditInput) (repository.CreateAPIKeyResult, error)
	UpdateAPIKeyStatusWithAudit(ctx context.Context, input repository.UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error)
	ListRechargeRequestsByWorkspace(ctx context.Context, workspaceID string, limit int) ([]domain.RechargeRequest, error)
	ListLedgerEntriesByWorkspace(ctx context.Context, workspaceID string, limit int) ([]domain.LedgerEntry, error)
	CreateRechargeRequest(ctx context.Context, input repository.CreateRechargeRequestInput) (domain.RechargeRequest, error)
}

type consoleService struct {
	store         consoleStore
	authPepper    string
	trialCredit   config.TrialCreditConfig
	runtimeSyncer APIKeyRuntimeSyncer
	usageService  *usage.Service
	nowFunc       func() time.Time
}

func newConsoleService(store consoleStore, authPepper string, trialCredit config.TrialCreditConfig) (*consoleService, error) {
	return newConsoleServiceWithRuntimeSyncer(store, authPepper, trialCredit, NewNoopAPIKeyRuntimeSyncer())
}

func newConsoleServiceWithRuntimeSyncer(store consoleStore, authPepper string, trialCredit config.TrialCreditConfig, runtimeSyncer APIKeyRuntimeSyncer) (*consoleService, error) {
	return newConsoleServiceWithRuntimeSyncerAndUsage(store, authPepper, trialCredit, runtimeSyncer, usage.NewService(usage.DisabledReader{}, nil))
}

func newConsoleServiceWithRuntimeSyncerAndUsage(store consoleStore, authPepper string, trialCredit config.TrialCreditConfig, runtimeSyncer APIKeyRuntimeSyncer, usageService *usage.Service) (*consoleService, error) {
	if store == nil {
		return nil, errors.New("console store is required")
	}
	if runtimeSyncer == nil {
		return nil, errors.New("api key runtime syncer is required")
	}
	if usageService == nil {
		usageService = usage.NewService(usage.DisabledReader{}, nil)
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
		store:         store,
		authPepper:    authPepper,
		trialCredit:   trialCredit,
		runtimeSyncer: runtimeSyncer,
		usageService:  usageService,
		nowFunc:       func() time.Time { return time.Now().UTC() },
	}, nil
}

func NewConsoleService(repos *repository.Repositories, authPepper string, trialCredit config.TrialCreditConfig) (ConsoleService, error) {
	if repos == nil {
		return nil, errors.New("console repositories are required")
	}
	return newConsoleService(repos, authPepper, trialCredit)
}

func NewConsoleServiceWithRuntimeSyncer(repos *repository.Repositories, authPepper string, trialCredit config.TrialCreditConfig, runtimeSyncer APIKeyRuntimeSyncer) (ConsoleService, error) {
	if repos == nil {
		return nil, errors.New("console repositories are required")
	}
	return newConsoleServiceWithRuntimeSyncer(repos, authPepper, trialCredit, runtimeSyncer)
}

func NewConsoleServiceWithRuntimeSyncerAndUsage(repos *repository.Repositories, authPepper string, trialCredit config.TrialCreditConfig, runtimeSyncer APIKeyRuntimeSyncer, usageService *usage.Service) (ConsoleService, error) {
	if repos == nil {
		return nil, errors.New("console repositories are required")
	}
	return newConsoleServiceWithRuntimeSyncerAndUsage(repos, authPepper, trialCredit, runtimeSyncer, usageService)
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
	runtimeActivated := workspaceRuntimeActivated(current.Workspace)
	firstCallMade := false

	return ConsoleOverviewResponse{
		Workspace: workspaceResponseFromRepository(current),
		Activation: ActivationOverviewResponse{
			TrialCreditGranted: trialGranted,
			TrialExpiresAt:     trialExpiresAt,
			APIKeyCreated:      apiKeyCreated,
			RuntimeActivated:   runtimeActivated,
			FirstCallMade:      firstCallMade,
			Steps:              activationSteps(trialGranted, apiKeyCreated, runtimeActivated, firstCallMade),
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

func (s *consoleService) BillingOverview(ctx context.Context, user CurrentUser) (BillingOverviewResponse, error) {
	current, err := s.resolveBillingWorkspace(ctx, user)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	requests, err := s.store.ListRechargeRequestsByWorkspace(ctx, current.Workspace.ID, rechargeRequestListLimit)
	if err != nil {
		return BillingOverviewResponse{}, mapConsoleRepositoryError(err)
	}
	entries, err := s.store.ListLedgerEntriesByWorkspace(ctx, current.Workspace.ID, ledgerEntryListLimit)
	if err != nil {
		return BillingOverviewResponse{}, mapConsoleRepositoryError(err)
	}

	resp := BillingOverviewResponse{
		Workspace:        workspaceResponseFromRepository(current),
		RechargeRequests: make([]RechargeRequestResponse, 0, len(requests)),
		LedgerEntries:    make([]LedgerEntryResponse, 0, len(entries)),
	}
	for _, request := range requests {
		resp.RechargeRequests = append(resp.RechargeRequests, rechargeRequestResponseFromDomain(request))
	}
	for _, entry := range entries {
		resp.LedgerEntries = append(resp.LedgerEntries, ledgerEntryResponseFromDomain(entry))
	}
	return resp, nil
}

func (s *consoleService) UsageSummary(ctx context.Context, user CurrentUser) (usage.SummaryResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return usage.SummaryResponse{}, err
	}
	return s.usageService.Summary(ctx, current.Workspace.ID)
}

func (s *consoleService) RequestLogs(ctx context.Context, user CurrentUser, limit int) (usage.RequestLogsResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return usage.RequestLogsResponse{}, err
	}
	return s.usageService.RecentLogs(ctx, current.Workspace.ID, limit)
}

func (s *consoleService) CreateRechargeRequest(ctx context.Context, user CurrentUser, input CreateRechargeRequestRequest) (CreateRechargeRequestResponse, error) {
	current, err := s.resolveBillingWorkspace(ctx, user)
	if err != nil {
		return CreateRechargeRequestResponse{}, err
	}
	createInput, err := s.validateCreateRechargeRequestInput(input)
	if err != nil {
		return CreateRechargeRequestResponse{}, err
	}
	createInput.WorkspaceID = current.Workspace.ID
	createInput.RequestedByUserID = user.ID

	request, err := s.store.CreateRechargeRequest(ctx, createInput)
	if err != nil {
		return CreateRechargeRequestResponse{}, mapConsoleRepositoryError(err)
	}
	return CreateRechargeRequestResponse{RechargeRequest: rechargeRequestResponseFromDomain(request)}, nil
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
	s.syncAPIKeyRuntimeBestEffort(ctx, current.Workspace, created.APIKey)
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
	s.syncAPIKeyRuntimeBestEffort(ctx, current.Workspace, updated)
	return apiKeyResponseFromDomain(updated), nil
}

func (s *consoleService) syncAPIKeyRuntimeBestEffort(ctx context.Context, workspace domain.Workspace, key domain.APIKey) {
	if err := s.syncAPIKeyRuntime(ctx, workspace, key); err != nil {
		log.Printf("portal api key runtime sync failed: workspace_id=%s api_key_id=%s err=%v", workspace.ID, key.ID, err)
	}
}

func (s *consoleService) syncAPIKeyRuntime(ctx context.Context, workspace domain.Workspace, key domain.APIKey) error {
	record, shouldUpsert := runtimeRecordFromAPIKey(workspace, key)
	if shouldUpsert {
		return s.runtimeSyncer.UpsertAPIKey(ctx, record)
	}
	if record.KeyHash == "" {
		return nil
	}
	return s.runtimeSyncer.DeleteAPIKey(ctx, record.KeyHash)
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

func (s *consoleService) resolveBillingWorkspace(ctx context.Context, user CurrentUser) (repository.CurrentWorkspaceResult, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return repository.CurrentWorkspaceResult{}, err
	}
	if !canManageBilling(current.Role) {
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
	case errors.Is(err, ErrConsoleRechargeInvalidAmount):
		return ErrInvalidRequest
	case errors.Is(err, ErrConsoleRechargeInvalidRequest):
		return ErrInvalidRequest
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

func activationSteps(trialGranted bool, apiKeyCreated bool, runtimeActivated bool, firstCallMade bool) []ActivationStepResponse {
	return []ActivationStepResponse{
		{Key: "trial_credit", Label: "Receive trial credit", Status: activationStatus(trialGranted)},
		{Key: "api_key", Label: "Create API key", Status: activationStatus(apiKeyCreated)},
		{Key: "runtime_activation", Label: "Activate runtime access", Status: activationStatus(runtimeActivated)},
		{Key: "first_call", Label: "Make first API call", Status: activationStatus(firstCallMade)},
	}
}

func workspaceRuntimeActivated(workspace domain.Workspace) bool {
	return workspace.TenantCode != nil && strings.TrimSpace(*workspace.TenantCode) != ""
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

func rechargeRequestResponseFromDomain(request domain.RechargeRequest) RechargeRequestResponse {
	return RechargeRequestResponse{
		ID:                request.ID,
		RequestedByUserID: request.RequestedByUserID,
		AmountMicroCNY:    request.AmountMicroCNY,
		AmountCNY:         money.MicroCNY(request.AmountMicroCNY).FormatCNY(),
		Currency:          request.Currency,
		Status:            request.Status,
		PaymentMethod:     request.PaymentMethod,
		Contact:           request.Contact,
		Note:              request.Note,
		AdminNote:         request.AdminNote,
		CreatedAt:         request.CreatedAt,
		UpdatedAt:         request.UpdatedAt,
	}
}

func ledgerEntryResponseFromDomain(entry domain.LedgerEntry) LedgerEntryResponse {
	return LedgerEntryResponse{
		ID:                       entry.ID,
		Type:                     entry.Type,
		Direction:                entry.Direction,
		AmountMicroCNY:           entry.AmountMicroCNY,
		AmountCNY:                money.MicroCNY(entry.AmountMicroCNY).FormatCNY(),
		BalanceAfterMicroCNY:     entry.BalanceAfterMicroCNY,
		BalanceAfterCNY:          money.MicroCNY(entry.BalanceAfterMicroCNY).FormatCNY(),
		Currency:                 entry.Currency,
		APIKeyID:                 entry.APIKeyID,
		APIKeyNameSnapshot:       entry.APIKeyNameSnapshot,
		ModelID:                  entry.ModelID,
		ModelDisplayNameSnapshot: entry.ModelDisplayNameSnapshot,
		CreatedAt:                entry.CreatedAt,
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

func canManageBilling(role domain.MemberRole) bool {
	return role == domain.MemberRoleOwner || role == domain.MemberRoleBilling
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

func (s *consoleService) validateCreateRechargeRequestInput(input CreateRechargeRequestRequest) (repository.CreateRechargeRequestInput, error) {
	paymentMethod := strings.TrimSpace(input.PaymentMethod)
	contact := strings.TrimSpace(input.Contact)
	note := strings.TrimSpace(input.Note)
	if input.AmountMicroCNY <= 0 {
		return repository.CreateRechargeRequestInput{}, ErrConsoleRechargeInvalidAmount
	}
	if paymentMethod == "" || utf8.RuneCountInString(paymentMethod) > maxRechargePaymentLength {
		return repository.CreateRechargeRequestInput{}, ErrConsoleRechargeInvalidRequest
	}
	if contact == "" || utf8.RuneCountInString(contact) > maxRechargeContactLength {
		return repository.CreateRechargeRequestInput{}, ErrConsoleRechargeInvalidRequest
	}
	if utf8.RuneCountInString(note) > maxRechargeNoteLength {
		return repository.CreateRechargeRequestInput{}, ErrConsoleRechargeInvalidRequest
	}

	return repository.CreateRechargeRequestInput{
		AmountMicroCNY: input.AmountMicroCNY,
		PaymentMethod:  paymentMethod,
		Contact:        contact,
		Note:           note,
	}, nil
}
