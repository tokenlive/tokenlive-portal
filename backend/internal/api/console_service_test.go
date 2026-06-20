package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

func TestMoneyAmountFromPointerFormatsCNY(t *testing.T) {
	t.Parallel()

	value := int64(1_230_000)
	got := moneyAmountFromPointer(&value)

	if got.MicroCNY == nil || *got.MicroCNY != value {
		t.Fatalf("micro cny = %v, want %d", got.MicroCNY, value)
	}
	if got.CNY == nil || *got.CNY != "1.230000" {
		t.Fatalf("cny = %v, want 1.230000", got.CNY)
	}
}

func TestMoneyAmountFromPointerKeepsNil(t *testing.T) {
	t.Parallel()

	got := moneyAmountFromPointer(nil)

	if got.MicroCNY != nil || got.CNY != nil {
		t.Fatalf("got %+v, want nil amount", got)
	}
}

func TestValidateCreateAPIKeyInputRejectsBadValues(t *testing.T) {
	t.Parallel()

	service := &consoleService{}
	negative := int64(-1)
	past := time.Now().UTC().Add(-time.Minute)

	tests := []struct {
		name  string
		input CreateAPIKeyRequest
		want  error
	}{
		{name: "blank name", input: CreateAPIKeyRequest{Name: "   "}, want: ErrConsoleAPIKeyInvalidName},
		{name: "long name", input: CreateAPIKeyRequest{Name: strings.Repeat("a", maxAPIKeyNameLength+1)}, want: ErrConsoleAPIKeyInvalidName},
		{name: "negative daily limit", input: CreateAPIKeyRequest{Name: "dev", DailyLimitMicroCNY: &negative}, want: ErrConsoleAPIKeyInvalidLimit},
		{name: "negative monthly limit", input: CreateAPIKeyRequest{Name: "dev", MonthlyLimitMicroCNY: &negative}, want: ErrConsoleAPIKeyInvalidLimit},
		{name: "past expiration", input: CreateAPIKeyRequest{Name: "dev", ExpiresAt: &past}, want: ErrConsoleAPIKeyInvalidExpiration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.validateCreateAPIKeyInput(tt.input, time.Now().UTC())
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateCreateAPIKeyInputAcceptsNonASCIINameAtLimit(t *testing.T) {
	t.Parallel()

	service := &consoleService{}
	name := strings.Repeat("钥", maxAPIKeyNameLength)

	got, err := service.validateCreateAPIKeyInput(CreateAPIKeyRequest{Name: name}, time.Now().UTC())
	if err != nil {
		t.Fatalf("validateCreateAPIKeyInput() err = %v, want nil", err)
	}
	if got.Name != name {
		t.Fatalf("name = %q, want %q", got.Name, name)
	}
}

func TestCanManageAPIKeys(t *testing.T) {
	t.Parallel()

	if !canManageAPIKeys(domain.MemberRoleOwner) {
		t.Fatalf("owner should manage api keys")
	}
	if !canManageAPIKeys(domain.MemberRoleDeveloper) {
		t.Fatalf("developer should manage api keys")
	}
	if canManageAPIKeys(domain.MemberRoleBilling) {
		t.Fatalf("billing should not manage api keys")
	}
}

func TestMapConsoleError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want AppError
	}{
		{name: "workspace not found", err: ErrConsoleWorkspaceNotFound, want: ErrWorkspaceNotFound},
		{name: "permission denied", err: ErrConsolePermissionDenied, want: ErrPermissionDenied},
		{name: "api key not found", err: ErrConsoleAPIKeyNotFound, want: ErrAPIKeyNotFound},
		{name: "api key invalid state", err: ErrConsoleAPIKeyInvalidState, want: ErrAPIKeyInvalidState},
		{name: "api key invalid name", err: ErrConsoleAPIKeyInvalidName, want: ErrAPIKeyInvalidName},
		{name: "api key invalid limit", err: ErrConsoleAPIKeyInvalidLimit, want: ErrAPIKeyInvalidLimit},
		{name: "api key invalid expiration", err: ErrConsoleAPIKeyInvalidExpiration, want: ErrAPIKeyInvalidExpiration},
		{name: "auth unauthorized", err: ErrAuthUnauthorized, want: ErrUnauthorized},
		{name: "wrapped permission denied", err: errors.Join(errors.New("wrapper"), ErrConsolePermissionDenied), want: ErrPermissionDenied},
		{name: "unknown", err: errors.New("boom"), want: ErrInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapConsoleError(tt.err); got != tt.want {
				t.Fatalf("mapConsoleError() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewConsoleServiceValidatesTrialCreditConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		trialCredit config.TrialCreditConfig
		wantErr     string
	}{
		{
			name:        "negative amount",
			trialCredit: config.TrialCreditConfig{AmountMicroCNY: -1, TTLDays: 7},
			wantErr:     "trial credit amount",
		},
		{
			name:        "zero ttl",
			trialCredit: config.TrialCreditConfig{AmountMicroCNY: 10_000_000, TTLDays: 0},
			wantErr:     "trial credit ttl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newConsoleService(&fakeConsoleStore{}, "pepper", tt.trialCredit)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestConsoleServiceCurrentWorkspaceMapsBalance(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
			},
			Role: domain.MemberRoleOwner,
			Balance: domain.WorkspaceBalance{
				AvailableMicroCNY: 12_340_000,
				FrozenMicroCNY:    500_000,
			},
		},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CurrentWorkspace(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("current workspace: %v", err)
	}

	if store.resolveUserID != "usr_1" {
		t.Fatalf("resolve user id = %q, want usr_1", store.resolveUserID)
	}
	if got.Workspace.ID != "wsp_1" || got.Workspace.Name != "Dev" || got.Workspace.Slug != "dev" {
		t.Fatalf("workspace = %+v", got.Workspace)
	}
	if got.Workspace.Role != domain.MemberRoleOwner || got.Workspace.Status != domain.WorkspaceStatusActive {
		t.Fatalf("workspace role/status = %s/%s", got.Workspace.Role, got.Workspace.Status)
	}
	if got.Workspace.TrialGrantedAt == nil || !got.Workspace.TrialGrantedAt.Equal(trialGrantedAt) {
		t.Fatalf("trial_granted_at = %v, want %v", got.Workspace.TrialGrantedAt, trialGrantedAt)
	}
	if got.Workspace.Balance.AvailableMicroCNY != 12_340_000 || got.Workspace.Balance.FrozenMicroCNY != 500_000 {
		t.Fatalf("balance micros = %+v", got.Workspace.Balance)
	}
	if got.Workspace.Balance.AvailableCNY != "12.340000" || got.Workspace.Balance.FrozenCNY != "0.500000" {
		t.Fatalf("balance cny = %+v", got.Workspace.Balance)
	}
}

func TestConsoleServiceBillingCannotListAPIKeys(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleBilling,
		},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1"})
	if !errors.Is(err, ErrConsolePermissionDenied) {
		t.Fatalf("err = %v, want ErrConsolePermissionDenied", err)
	}
	if store.listWorkspaceID != "" {
		t.Fatalf("list workspace id = %q, want no repository list call", store.listWorkspaceID)
	}
}

func TestConsoleServiceCreateAPIKeyReturnsSecretOnce(t *testing.T) {
	t.Parallel()

	dailyLimit := int64(5_000_000)
	monthlyLimit := int64(50_000_000)
	expiresAt := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleDeveloper,
		},
		createAPIKeyResult: repository.CreateAPIKeyResult{
			APIKey: domain.APIKey{
				ID:                   "ak_1",
				WorkspaceID:          "wsp_1",
				Name:                 "local dev",
				KeyPrefix:            "tl_live_abc",
				SecretLast4:          "wxyz",
				KeyHash:              "hash-should-not-appear",
				Status:               domain.APIKeyStatusEnabled,
				CreatedByUserID:      "usr_1",
				ExpiresAt:            &expiresAt,
				DailyLimitMicroCNY:   &dailyLimit,
				MonthlyLimitMicroCNY: &monthlyLimit,
				TotalSpendMicroCNY:   1_250_000,
				CreatedAt:            createdAt,
				UpdatedAt:            createdAt,
			},
			Secret: "tl_live_secret",
		},
	}
	service, err := newConsoleService(store, " pepper ", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1"}, CreateAPIKeyRequest{
		Name:                 " local dev ",
		DailyLimitMicroCNY:   &dailyLimit,
		MonthlyLimitMicroCNY: &monthlyLimit,
		ExpiresAt:            &expiresAt,
	})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if got.Secret != "tl_live_secret" {
		t.Fatalf("secret = %q, want creation secret", got.Secret)
	}
	if got.APIKey.ID != "ak_1" || got.APIKey.SecretLast4 != "wxyz" {
		t.Fatalf("api key response = %+v", got.APIKey)
	}
	if got.APIKey.DailyLimitCNY == nil || *got.APIKey.DailyLimitCNY != "5.000000" {
		t.Fatalf("daily limit cny = %v, want 5.000000", got.APIKey.DailyLimitCNY)
	}
	if got.APIKey.MonthlyLimitCNY == nil || *got.APIKey.MonthlyLimitCNY != "50.000000" {
		t.Fatalf("monthly limit cny = %v, want 50.000000", got.APIKey.MonthlyLimitCNY)
	}
	if got.APIKey.TotalSpendCNY != "1.250000" {
		t.Fatalf("total spend cny = %q, want 1.250000", got.APIKey.TotalSpendCNY)
	}
	if store.createAPIKeyInput.WorkspaceID != "wsp_1" {
		t.Fatalf("workspace id = %s, want wsp_1", store.createAPIKeyInput.WorkspaceID)
	}
	if store.createAPIKeyInput.CreatedByUserID != "usr_1" || store.createAPIKeyInput.ActorUserID != "usr_1" {
		t.Fatalf("user ids = created_by:%s actor:%s, want usr_1", store.createAPIKeyInput.CreatedByUserID, store.createAPIKeyInput.ActorUserID)
	}
	if store.createAPIKeyInput.Name != "local dev" {
		t.Fatalf("name = %q, want trimmed name", store.createAPIKeyInput.Name)
	}
	if store.createAPIKeyInput.Pepper != " pepper " {
		t.Fatalf("pepper = %q, want constructor pepper", store.createAPIKeyInput.Pepper)
	}
	if store.createAPIKeyInput.PlaintextKey != "" {
		t.Fatalf("plaintext key = %q, want repository to generate it", store.createAPIKeyInput.PlaintextKey)
	}
}

func TestConsoleServiceListAPIKeysDoesNotExposeSecret(t *testing.T) {
	t.Parallel()

	dailyLimit := int64(5_000_000)
	createdAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		listAPIKeysResult: []domain.APIKey{{
			ID:                 "ak_1",
			Name:               "local dev",
			KeyPrefix:          "tl_live_abc",
			SecretLast4:        "wxyz",
			KeyHash:            "hash-should-not-appear",
			Status:             domain.APIKeyStatusEnabled,
			CreatedByUserID:    "usr_1",
			DailyLimitMicroCNY: &dailyLimit,
			TotalSpendMicroCNY: 750_000,
			CreatedAt:          createdAt,
			UpdatedAt:          createdAt,
		}},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}

	if store.listWorkspaceID != "wsp_1" {
		t.Fatalf("list workspace id = %q, want wsp_1", store.listWorkspaceID)
	}
	if len(got.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(got.Data))
	}
	if got.Data[0].SecretLast4 != "wxyz" || got.Data[0].KeyPrefix != "tl_live_abc" {
		t.Fatalf("api key response = %+v", got.Data[0])
	}
	if got.Data[0].DailyLimitCNY == nil || *got.Data[0].DailyLimitCNY != "5.000000" {
		t.Fatalf("daily limit cny = %v, want 5.000000", got.Data[0].DailyLimitCNY)
	}
	if got.Data[0].TotalSpendCNY != "0.750000" {
		t.Fatalf("total spend cny = %q, want 0.750000", got.Data[0].TotalSpendCNY)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	bodyString := string(body)
	if strings.Contains(bodyString, "hash-should-not-appear") || strings.Contains(bodyString, "key_hash") {
		t.Fatalf("list response exposed key hash: %s", bodyString)
	}
	if strings.Contains(bodyString, "tl_live_secret") || strings.Contains(bodyString, `"secret"`) {
		t.Fatalf("list response exposed creation secret field: %s", bodyString)
	}
}

func TestConsoleServiceStateUpdatesUseCurrentWorkspaceAndTargetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		call   func(*consoleService, context.Context, CurrentUser, string) (APIKeyResponse, error)
		status domain.APIKeyStatus
	}{
		{name: "enable", call: (*consoleService).EnableAPIKey, status: domain.APIKeyStatusEnabled},
		{name: "disable", call: (*consoleService).DisableAPIKey, status: domain.APIKeyStatusDisabled},
		{name: "revoke", call: (*consoleService).RevokeAPIKey, status: domain.APIKeyStatusRevoked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeConsoleStore{
				currentWorkspace: repository.CurrentWorkspaceResult{
					Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
					Role:      domain.MemberRoleDeveloper,
				},
				updateAPIKeyResult: domain.APIKey{
					ID:          "ak_1",
					WorkspaceID: "wsp_1",
					Name:        "local dev",
					KeyPrefix:   "tl_live_abc",
					SecretLast4: "wxyz",
					Status:      tt.status,
				},
			}
			service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
			if err != nil {
				t.Fatalf("new console service: %v", err)
			}

			got, err := tt.call(service, context.Background(), CurrentUser{ID: "usr_1"}, " ak_1 ")
			if err != nil {
				t.Fatalf("%s api key: %v", tt.name, err)
			}

			if got.Status != tt.status {
				t.Fatalf("status = %s, want %s", got.Status, tt.status)
			}
			if store.updateAPIKeyInput.WorkspaceID != "wsp_1" {
				t.Fatalf("workspace id = %s, want wsp_1", store.updateAPIKeyInput.WorkspaceID)
			}
			if store.updateAPIKeyInput.APIKeyID != "ak_1" {
				t.Fatalf("api key id = %q, want trimmed ak_1", store.updateAPIKeyInput.APIKeyID)
			}
			if store.updateAPIKeyInput.Status != tt.status {
				t.Fatalf("status = %s, want %s", store.updateAPIKeyInput.Status, tt.status)
			}
			if store.updateAPIKeyInput.ActorUserID != "usr_1" {
				t.Fatalf("actor user id = %s, want usr_1", store.updateAPIKeyInput.ActorUserID)
			}
		})
	}
}

func TestConsoleServiceOverviewMapsActivationState(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
			},
			Role: domain.MemberRoleBilling,
			Balance: domain.WorkspaceBalance{
				AvailableMicroCNY: 10_000_000,
			},
		},
		listAPIKeysResult: []domain.APIKey{{
			ID:     "ak_1",
			Status: domain.APIKeyStatusRevoked,
		}},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.Overview(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	if store.listWorkspaceID != "wsp_1" {
		t.Fatalf("list workspace id = %q, want wsp_1", store.listWorkspaceID)
	}
	if got.Workspace.TrialGrantedAt == nil || !got.Workspace.TrialGrantedAt.Equal(trialGrantedAt) {
		t.Fatalf("trial_granted_at = %v, want %v", got.Workspace.TrialGrantedAt, trialGrantedAt)
	}
	if got.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("available cny = %s, want 10.000000", got.Workspace.Balance.AvailableCNY)
	}
	if !got.Activation.TrialCreditGranted {
		t.Fatalf("expected trial credit granted")
	}
	wantExpiresAt := trialGrantedAt.AddDate(0, 0, 7)
	if got.Activation.TrialExpiresAt == nil || !got.Activation.TrialExpiresAt.Equal(wantExpiresAt) {
		t.Fatalf("trial expires at = %v, want %v", got.Activation.TrialExpiresAt, wantExpiresAt)
	}
	if !got.Activation.APIKeyCreated {
		t.Fatalf("expected api key created")
	}
	if got.Activation.FirstCallMade {
		t.Fatalf("first_call_made should remain false until usage slice")
	}
	if len(got.Activation.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(got.Activation.Steps))
	}
	if got.Activation.Steps[0].Key != "trial_credit" || got.Activation.Steps[0].Status != ActivationStepCompleted {
		t.Fatalf("trial step = %+v", got.Activation.Steps[0])
	}
	if got.Activation.Steps[1].Key != "api_key" || got.Activation.Steps[1].Status != ActivationStepCompleted {
		t.Fatalf("api key step = %+v", got.Activation.Steps[1])
	}
	if got.Activation.Steps[2].Key != "first_call" || got.Activation.Steps[2].Status != ActivationStepPending {
		t.Fatalf("first call step = %+v", got.Activation.Steps[2])
	}
}

func TestConsoleServiceOverviewRequiresUser(t *testing.T) {
	t.Parallel()

	service, err := newConsoleService(&fakeConsoleStore{}, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.Overview(context.Background(), CurrentUser{})
	if !errors.Is(err, ErrAuthUnauthorized) {
		t.Fatalf("err = %v, want ErrAuthUnauthorized", err)
	}
}

func TestConsoleServiceOverviewMapsWorkspaceNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{currentWorkspaceErr: repository.ErrWorkspaceNotFound}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.Overview(context.Background(), CurrentUser{ID: "usr_1"})
	if !errors.Is(err, ErrConsoleWorkspaceNotFound) {
		t.Fatalf("err = %v, want ErrConsoleWorkspaceNotFound", err)
	}
}

func testTrialCreditConfig() config.TrialCreditConfig {
	return config.TrialCreditConfig{
		AmountMicroCNY: 10_000_000,
		TTLDays:        7,
	}
}

type fakeConsoleStore struct {
	currentWorkspace    repository.CurrentWorkspaceResult
	currentWorkspaceErr error
	resolveUserID       string

	listWorkspaceID   string
	listAPIKeysResult []domain.APIKey
	listAPIKeysErr    error

	createAPIKeyInput  repository.CreateAPIKeyWithAuditInput
	createAPIKeyResult repository.CreateAPIKeyResult
	createAPIKeyErr    error

	updateAPIKeyInput  repository.UpdateAPIKeyStatusWithAuditInput
	updateAPIKeyResult domain.APIKey
	updateAPIKeyErr    error
}

func (f *fakeConsoleStore) ResolveCurrentWorkspace(_ context.Context, userID string) (repository.CurrentWorkspaceResult, error) {
	f.resolveUserID = userID
	if f.currentWorkspaceErr != nil {
		return repository.CurrentWorkspaceResult{}, f.currentWorkspaceErr
	}
	return f.currentWorkspace, nil
}

func (f *fakeConsoleStore) ListAPIKeysByWorkspace(_ context.Context, workspaceID string) ([]domain.APIKey, error) {
	f.listWorkspaceID = workspaceID
	if f.listAPIKeysErr != nil {
		return nil, f.listAPIKeysErr
	}
	return f.listAPIKeysResult, nil
}

func (f *fakeConsoleStore) CreateAPIKeyWithAudit(_ context.Context, input repository.CreateAPIKeyWithAuditInput) (repository.CreateAPIKeyResult, error) {
	f.createAPIKeyInput = input
	if f.createAPIKeyErr != nil {
		return repository.CreateAPIKeyResult{}, f.createAPIKeyErr
	}
	return f.createAPIKeyResult, nil
}

func (f *fakeConsoleStore) UpdateAPIKeyStatusWithAudit(_ context.Context, input repository.UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error) {
	f.updateAPIKeyInput = input
	if f.updateAPIKeyErr != nil {
		return domain.APIKey{}, f.updateAPIKeyErr
	}
	return f.updateAPIKeyResult, nil
}
