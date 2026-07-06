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

func TestCanManageBilling(t *testing.T) {
	t.Parallel()

	if !canManageBilling(domain.MemberRoleOwner) {
		t.Fatalf("owner should manage billing")
	}
	if !canManageBilling(domain.MemberRoleBilling) {
		t.Fatalf("billing should manage billing")
	}
	if canManageBilling(domain.MemberRoleDeveloper) {
		t.Fatalf("developer should not manage billing")
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
		{name: "recharge invalid amount", err: ErrConsoleRechargeInvalidAmount, want: ErrInvalidRequest},
		{name: "recharge invalid request", err: ErrConsoleRechargeInvalidRequest, want: ErrInvalidRequest},
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

func TestValidateCreateRechargeRequestInputRejectsBadValues(t *testing.T) {
	t.Parallel()

	service := &consoleService{}

	tests := []struct {
		name  string
		input CreateRechargeRequestRequest
		want  error
	}{
		{name: "zero amount", input: CreateRechargeRequestRequest{AmountMicroCNY: 0, PaymentMethod: "bank_transfer", Contact: "ops@example.com"}, want: ErrConsoleRechargeInvalidAmount},
		{name: "negative amount", input: CreateRechargeRequestRequest{AmountMicroCNY: -1, PaymentMethod: "bank_transfer", Contact: "ops@example.com"}, want: ErrConsoleRechargeInvalidAmount},
		{name: "blank payment", input: CreateRechargeRequestRequest{AmountMicroCNY: 1, Contact: "ops@example.com"}, want: ErrConsoleRechargeInvalidRequest},
		{name: "blank contact", input: CreateRechargeRequestRequest{AmountMicroCNY: 1, PaymentMethod: "bank_transfer"}, want: ErrConsoleRechargeInvalidRequest},
		{name: "long note", input: CreateRechargeRequestRequest{AmountMicroCNY: 1, PaymentMethod: "bank_transfer", Contact: "ops@example.com", Note: strings.Repeat("a", maxRechargeNoteLength+1)}, want: ErrConsoleRechargeInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.validateCreateRechargeRequestInput(tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
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

	got, err := service.CurrentWorkspace(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
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

	_, err = service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
	if !errors.Is(err, ErrConsolePermissionDenied) {
		t.Fatalf("err = %v, want ErrConsolePermissionDenied", err)
	}
	if store.listWorkspaceID != "" {
		t.Fatalf("list workspace id = %q, want no repository list call", store.listWorkspaceID)
	}
}

func TestConsoleServiceBillingOverviewListsRechargeRequests(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Name: "Dev", Slug: "dev", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleBilling,
			Balance:   domain.WorkspaceBalance{AvailableMicroCNY: 10_000_000},
		},
		listRechargeRequestsResult: []domain.RechargeRequest{{
			ID:                "rch_1",
			WorkspaceID:       "wsp_1",
			RequestedByUserID: "usr_1",
			AmountMicroCNY:    20_000_000,
			Currency:          "CNY",
			Status:            domain.RechargeRequestStatusPending,
			PaymentMethod:     "bank_transfer",
			Contact:           "ops@example.com",
			Note:              "invoice needed",
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		}},
		listLedgerEntriesResult: []domain.LedgerEntry{{
			ID:                   "led_1",
			WorkspaceID:          "wsp_1",
			Type:                 domain.LedgerTypeTrialGrant,
			Direction:            domain.LedgerDirectionCredit,
			AmountMicroCNY:       10_000_000,
			BalanceAfterMicroCNY: 10_000_000,
			Currency:             "CNY",
			IdempotencyKey:       "trial-grant:wsp_1",
			CreatedAt:            createdAt,
		}},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.BillingOverview(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
	if err != nil {
		t.Fatalf("billing overview: %v", err)
	}

	if store.listRechargeWorkspaceID != "wsp_1" {
		t.Fatalf("list recharge workspace id = %q, want wsp_1", store.listRechargeWorkspaceID)
	}
	if store.listLedgerWorkspaceID != "wsp_1" {
		t.Fatalf("list ledger workspace id = %q, want wsp_1", store.listLedgerWorkspaceID)
	}
	if got.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("available cny = %q, want 10.000000", got.Workspace.Balance.AvailableCNY)
	}
	if len(got.RechargeRequests) != 1 || got.RechargeRequests[0].ID != "rch_1" || got.RechargeRequests[0].AmountCNY != "20.000000" {
		t.Fatalf("recharge requests = %+v", got.RechargeRequests)
	}
	if len(got.LedgerEntries) != 1 || got.LedgerEntries[0].ID != "led_1" || got.LedgerEntries[0].AmountCNY != "10.000000" || got.LedgerEntries[0].BalanceAfterCNY != "10.000000" {
		t.Fatalf("ledger entries = %+v", got.LedgerEntries)
	}
}

func TestConsoleServiceDeveloperCannotCreateRechargeRequest(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleDeveloper,
		},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.CreateRechargeRequest(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateRechargeRequestRequest{
		AmountMicroCNY: 10_000_000,
		PaymentMethod:  "bank_transfer",
		Contact:        "ops@example.com",
	})
	if !errors.Is(err, ErrConsolePermissionDenied) {
		t.Fatalf("err = %v, want ErrConsolePermissionDenied", err)
	}
	if store.createRechargeRequestInput.WorkspaceID != "" {
		t.Fatalf("create recharge should not be called")
	}
}

func TestConsoleServiceCreateRechargeRequestReturnsPendingRequest(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		createRechargeRequestResult: domain.RechargeRequest{
			ID:                "rch_1",
			WorkspaceID:       "wsp_1",
			RequestedByUserID: "usr_1",
			AmountMicroCNY:    30_000_000,
			Currency:          "CNY",
			Status:            domain.RechargeRequestStatusPending,
			PaymentMethod:     "bank_transfer",
			Contact:           "ops@example.com",
			Note:              "top up",
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CreateRechargeRequest(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateRechargeRequestRequest{
		AmountMicroCNY: 30_000_000,
		PaymentMethod:  " bank_transfer ",
		Contact:        " ops@example.com ",
		Note:           " top up ",
	})
	if err != nil {
		t.Fatalf("create recharge request: %v", err)
	}

	if store.createRechargeRequestInput.WorkspaceID != "wsp_1" || store.createRechargeRequestInput.RequestedByUserID != "usr_1" {
		t.Fatalf("create input workspace/user = %+v", store.createRechargeRequestInput)
	}
	if store.createRechargeRequestInput.AmountMicroCNY != 30_000_000 || store.createRechargeRequestInput.PaymentMethod != "bank_transfer" || store.createRechargeRequestInput.Contact != "ops@example.com" || store.createRechargeRequestInput.Note != "top up" {
		t.Fatalf("create input = %+v", store.createRechargeRequestInput)
	}
	if got.RechargeRequest.ID != "rch_1" || got.RechargeRequest.AmountCNY != "30.000000" || got.RechargeRequest.Status != domain.RechargeRequestStatusPending {
		t.Fatalf("response = %+v", got.RechargeRequest)
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

	got, err := service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateAPIKeyRequest{
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

func TestConsoleServiceCreateAPIKeySyncsRuntimeForActiveRuntimeAccess(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		runtimeAccess: domain.WorkspaceRuntimeAccess{WorkspaceID: "wsp_1", ScopeType: domain.RuntimeAccessScopeTenant, ScopeCode: "tenant_a", Status: domain.RuntimeAccessStatusActive},
		createAPIKeyResult: repository.CreateAPIKeyResult{
			APIKey: domain.APIKey{
				ID:              "ak_1",
				WorkspaceID:     "wsp_1",
				Name:            "prod",
				KeyHash:         "hash_1",
				Status:          domain.APIKeyStatusEnabled,
				CreatedByUserID: "usr_1",
				ExpiresAt:       &expiresAt,
			},
			Secret: "tl_live_secret",
		},
	}
	syncer := &fakeAPIKeyRuntimeSyncer{}
	service, err := newConsoleServiceWithRuntimeSyncer(store, "pepper", testTrialCreditConfig(), syncer)
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateAPIKeyRequest{Name: "prod"})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if syncer.upsert.KeyHash != "hash_1" || syncer.upsert.KeyID != "ak_1" {
		t.Fatalf("upsert key identity = %+v", syncer.upsert)
	}
	if syncer.upsert.UserID != "usr_1" || syncer.upsert.WorkspaceID != "wsp_1" {
		t.Fatalf("upsert user/workspace = %+v", syncer.upsert)
	}
	if syncer.upsert.ScopeType != "tenant" || syncer.upsert.ScopeCode != "tenant_a" {
		t.Fatalf("upsert scope fields = %+v", syncer.upsert)
	}
	if syncer.upsert.Status != 1 || syncer.upsert.Quota != -1 || syncer.upsert.ExpiresAt != expiresAt.Unix() {
		t.Fatalf("upsert runtime fields = %+v", syncer.upsert)
	}
	if syncer.deletedHash != "" {
		t.Fatalf("deleted hash = %q, want none", syncer.deletedHash)
	}
}

func TestConsoleServiceCreateAPIKeyDeletesRuntimeWhenRuntimeAccessInactive(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		createAPIKeyResult: repository.CreateAPIKeyResult{
			APIKey: domain.APIKey{
				ID:              "ak_1",
				WorkspaceID:     "wsp_1",
				KeyHash:         "hash_1",
				Status:          domain.APIKeyStatusEnabled,
				CreatedByUserID: "usr_1",
			},
			Secret: "tl_live_secret",
		},
	}
	syncer := &fakeAPIKeyRuntimeSyncer{}
	service, err := newConsoleServiceWithRuntimeSyncer(store, "pepper", testTrialCreditConfig(), syncer)
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateAPIKeyRequest{Name: "prod"})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if syncer.deletedHash != "hash_1" {
		t.Fatalf("deleted hash = %q, want hash_1", syncer.deletedHash)
	}
	if syncer.upsert.KeyHash != "" {
		t.Fatalf("upsert = %+v, want none", syncer.upsert)
	}
}

func TestConsoleServiceCreateAPIKeyIgnoresRuntimeSyncFailure(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		runtimeAccess: domain.WorkspaceRuntimeAccess{WorkspaceID: "wsp_1", ScopeType: domain.RuntimeAccessScopeTenant, ScopeCode: "tenant_a", Status: domain.RuntimeAccessStatusActive},
		createAPIKeyResult: repository.CreateAPIKeyResult{
			APIKey: domain.APIKey{
				ID:              "ak_1",
				WorkspaceID:     "wsp_1",
				KeyHash:         "hash_1",
				Status:          domain.APIKeyStatusEnabled,
				CreatedByUserID: "usr_1",
			},
			Secret: "tl_live_secret",
		},
	}
	syncer := &fakeAPIKeyRuntimeSyncer{upsertErr: errors.New("redis down")}
	service, err := newConsoleServiceWithRuntimeSyncer(store, "pepper", testTrialCreditConfig(), syncer)
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, CreateAPIKeyRequest{Name: "prod"})
	if err != nil {
		t.Fatalf("create api key err = %v, want nil", err)
	}
	if got.APIKey.ID != "ak_1" || got.Secret != "tl_live_secret" {
		t.Fatalf("response = %+v", got)
	}
	if syncer.upsert.KeyHash != "hash_1" {
		t.Fatalf("upsert key hash = %q, want attempted sync", syncer.upsert.KeyHash)
	}
}

func TestConsoleServiceAPIKeyStatusUpdateSyncsRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		call       func(*consoleService, context.Context, CurrentUser, string) (APIKeyResponse, error)
		status     domain.APIKeyStatus
		wantUpsert bool
	}{
		{name: "enable upserts", call: (*consoleService).EnableAPIKey, status: domain.APIKeyStatusEnabled, wantUpsert: true},
		{name: "disable deletes", call: (*consoleService).DisableAPIKey, status: domain.APIKeyStatusDisabled},
		{name: "revoke deletes", call: (*consoleService).RevokeAPIKey, status: domain.APIKeyStatusRevoked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeConsoleStore{
				currentWorkspace: repository.CurrentWorkspaceResult{
					Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
					Role:      domain.MemberRoleOwner,
				},
				runtimeAccess: domain.WorkspaceRuntimeAccess{WorkspaceID: "wsp_1", ScopeType: domain.RuntimeAccessScopeTenant, ScopeCode: "tenant_a", Status: domain.RuntimeAccessStatusActive},
				updateAPIKeyResult: domain.APIKey{
					ID:              "ak_1",
					WorkspaceID:     "wsp_1",
					KeyHash:         "hash_1",
					Status:          tt.status,
					CreatedByUserID: "usr_1",
				},
			}
			syncer := &fakeAPIKeyRuntimeSyncer{}
			service, err := newConsoleServiceWithRuntimeSyncer(store, "pepper", testTrialCreditConfig(), syncer)
			if err != nil {
				t.Fatalf("new console service: %v", err)
			}

			_, err = tt.call(service, context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, "ak_1")
			if err != nil {
				t.Fatalf("update api key: %v", err)
			}

			if tt.wantUpsert {
				if syncer.upsert.KeyHash != "hash_1" || syncer.upsert.Status != 1 {
					t.Fatalf("upsert = %+v, want enabled runtime key", syncer.upsert)
				}
				if syncer.deletedHash != "" {
					t.Fatalf("deleted hash = %q, want none", syncer.deletedHash)
				}
				return
			}
			if syncer.deletedHash != "hash_1" {
				t.Fatalf("deleted hash = %q, want hash_1", syncer.deletedHash)
			}
			if syncer.upsert.KeyHash != "" {
				t.Fatalf("upsert = %+v, want none", syncer.upsert)
			}
		})
	}
}

func TestConsoleServiceStatusUpdateIgnoresRuntimeSyncFailure(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		runtimeAccess: domain.WorkspaceRuntimeAccess{WorkspaceID: "wsp_1", ScopeType: domain.RuntimeAccessScopeTenant, ScopeCode: "tenant_a", Status: domain.RuntimeAccessStatusActive},
		updateAPIKeyResult: domain.APIKey{
			ID:              "ak_1",
			WorkspaceID:     "wsp_1",
			KeyHash:         "hash_1",
			Status:          domain.APIKeyStatusDisabled,
			CreatedByUserID: "usr_1",
		},
	}
	syncer := &fakeAPIKeyRuntimeSyncer{deleteErr: errors.New("redis down")}
	service, err := newConsoleServiceWithRuntimeSyncer(store, "pepper", testTrialCreditConfig(), syncer)
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.DisableAPIKey(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, "ak_1")
	if err != nil {
		t.Fatalf("disable api key err = %v, want nil", err)
	}
	if got.ID != "ak_1" || got.Status != domain.APIKeyStatusDisabled {
		t.Fatalf("response = %+v", got)
	}
	if syncer.deletedHash != "hash_1" {
		t.Fatalf("deleted hash = %q, want attempted delete", syncer.deletedHash)
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

	got, err := service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
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

			got, err := tt.call(service, context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true}, " ak_1 ")
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
		runtimeAccess: domain.WorkspaceRuntimeAccess{WorkspaceID: "wsp_1", ScopeType: domain.RuntimeAccessScopeTenant, ScopeCode: "tenant_a", Status: domain.RuntimeAccessStatusActive},
	}
	service, err := newConsoleService(store, "pepper", testTrialCreditConfig())
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.Overview(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
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
	if !got.Activation.RuntimeActivated {
		t.Fatalf("expected runtime activated when runtime access is active")
	}
	if got.Activation.FirstCallMade {
		t.Fatalf("first_call_made should remain false until usage slice")
	}
	if len(got.Activation.Steps) != 4 {
		t.Fatalf("steps len = %d, want 4", len(got.Activation.Steps))
	}
	if got.Activation.Steps[0].Key != "trial_credit" || got.Activation.Steps[0].Status != ActivationStepCompleted {
		t.Fatalf("trial step = %+v", got.Activation.Steps[0])
	}
	if got.Activation.Steps[1].Key != "api_key" || got.Activation.Steps[1].Status != ActivationStepCompleted {
		t.Fatalf("api key step = %+v", got.Activation.Steps[1])
	}
	if got.Activation.Steps[2].Key != "runtime_activation" || got.Activation.Steps[2].Status != ActivationStepCompleted {
		t.Fatalf("runtime activation step = %+v", got.Activation.Steps[2])
	}
	if got.Activation.Steps[3].Key != "first_call" || got.Activation.Steps[3].Status != ActivationStepPending {
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

	_, err = service.Overview(context.Background(), CurrentUser{ID: "usr_1", TermsAccepted: true})
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
	runtimeAccess     domain.WorkspaceRuntimeAccess
	runtimeAccessErr  error

	createAPIKeyInput  repository.CreateAPIKeyWithAuditInput
	createAPIKeyResult repository.CreateAPIKeyResult
	createAPIKeyErr    error

	updateAPIKeyInput  repository.UpdateAPIKeyStatusWithAuditInput
	updateAPIKeyResult domain.APIKey
	updateAPIKeyErr    error

	listRechargeWorkspaceID     string
	listRechargeRequestsResult  []domain.RechargeRequest
	listRechargeRequestsErr     error
	listLedgerWorkspaceID       string
	listLedgerEntriesResult     []domain.LedgerEntry
	listLedgerEntriesErr        error
	createRechargeRequestInput  repository.CreateRechargeRequestInput
	createRechargeRequestResult domain.RechargeRequest
	createRechargeRequestErr    error
}

func (f *fakeConsoleStore) ResolveCurrentWorkspace(_ context.Context, userID string) (repository.CurrentWorkspaceResult, error) {
	f.resolveUserID = userID
	if f.currentWorkspaceErr != nil {
		return repository.CurrentWorkspaceResult{}, f.currentWorkspaceErr
	}
	return f.currentWorkspace, nil
}

func (f *fakeConsoleStore) FindWorkspaceRuntimeAccess(context.Context, string) (domain.WorkspaceRuntimeAccess, error) {
	if f.runtimeAccessErr != nil {
		return domain.WorkspaceRuntimeAccess{}, f.runtimeAccessErr
	}
	if f.runtimeAccess.WorkspaceID == "" {
		return domain.WorkspaceRuntimeAccess{}, repository.ErrWorkspaceRuntimeAccessNotFound
	}
	return f.runtimeAccess, nil
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

func (f *fakeConsoleStore) ListRechargeRequestsByWorkspace(_ context.Context, workspaceID string, _ int) ([]domain.RechargeRequest, error) {
	f.listRechargeWorkspaceID = workspaceID
	if f.listRechargeRequestsErr != nil {
		return nil, f.listRechargeRequestsErr
	}
	return f.listRechargeRequestsResult, nil
}

func (f *fakeConsoleStore) ListLedgerEntriesByWorkspace(_ context.Context, workspaceID string, _ int) ([]domain.LedgerEntry, error) {
	f.listLedgerWorkspaceID = workspaceID
	if f.listLedgerEntriesErr != nil {
		return nil, f.listLedgerEntriesErr
	}
	return f.listLedgerEntriesResult, nil
}

func (f *fakeConsoleStore) CreateRechargeRequest(_ context.Context, input repository.CreateRechargeRequestInput) (domain.RechargeRequest, error) {
	f.createRechargeRequestInput = input
	if f.createRechargeRequestErr != nil {
		return domain.RechargeRequest{}, f.createRechargeRequestErr
	}
	return f.createRechargeRequestResult, nil
}

type fakeAPIKeyRuntimeSyncer struct {
	upsert      APIKeyRuntimeRecord
	deletedHash string
	upsertErr   error
	deleteErr   error
}

func (f *fakeAPIKeyRuntimeSyncer) UpsertAPIKey(_ context.Context, record APIKeyRuntimeRecord) error {
	f.upsert = record
	return f.upsertErr
}

func (f *fakeAPIKeyRuntimeSyncer) DeleteAPIKey(_ context.Context, keyHash string) error {
	f.deletedHash = keyHash
	return f.deleteErr
}
