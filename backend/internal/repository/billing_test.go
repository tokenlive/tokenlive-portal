package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestCreateLedgerEntryIdempotent(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "billing-user-" + suffix + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Billing User",
		PrimaryEmail:  &email,
		WorkspaceName: "Billing Workspace",
		WorkspaceSlug: "billing-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	input := CreateLedgerInput{
		WorkspaceID:    result.Workspace.ID,
		Type:           domain.LedgerTypeTrialGrant,
		Direction:      domain.LedgerDirectionCredit,
		AmountMicroCNY: 1_000_000,
		Currency:       "CNY",
		IdempotencyKey: "trial-grant:" + suffix,
	}

	first, err := repos.CreateLedgerEntry(ctx, input)
	if err != nil {
		t.Fatalf("create first ledger: %v", err)
	}

	second, err := repos.CreateLedgerEntry(ctx, input)
	if err != nil {
		t.Fatalf("replay ledger: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay to return same ledger")
	}

	input.AmountMicroCNY = 2_000_000
	_, err = repos.CreateLedgerEntry(ctx, input)
	if !errors.Is(err, ErrDuplicateLedgerConflict) {
		t.Fatalf("got err %v, want ErrDuplicateLedgerConflict", err)
	}
}

func TestListLedgerEntriesByWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "ledger-list-user-" + suffix + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Ledger List User",
		PrimaryEmail:  &email,
		WorkspaceName: "Ledger List Workspace",
		WorkspaceSlug: "ledger-list-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	first, err := repos.CreateLedgerEntry(ctx, CreateLedgerInput{
		WorkspaceID:    result.Workspace.ID,
		Type:           domain.LedgerTypeTrialGrant,
		Direction:      domain.LedgerDirectionCredit,
		AmountMicroCNY: 1_000_000,
		Currency:       "CNY",
		IdempotencyKey: "ledger-list:first:" + suffix,
	})
	if err != nil {
		t.Fatalf("create first ledger: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := repos.CreateLedgerEntry(ctx, CreateLedgerInput{
		WorkspaceID:    result.Workspace.ID,
		Type:           domain.LedgerTypeAdjustment,
		Direction:      domain.LedgerDirectionDebit,
		AmountMicroCNY: 500_000,
		Currency:       "CNY",
		IdempotencyKey: "ledger-list:second:" + suffix,
	})
	if err != nil {
		t.Fatalf("create second ledger: %v", err)
	}

	entries, err := repos.ListLedgerEntriesByWorkspace(ctx, result.Workspace.ID, 10)
	if err != nil {
		t.Fatalf("list ledger entries: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].ID != second.ID || entries[1].ID != first.ID {
		t.Fatalf("entries order = [%s %s], want newest first [%s %s]", entries[0].ID, entries[1].ID, second.ID, first.ID)
	}
	if entries[0].BalanceAfterMicroCNY != 500_000 {
		t.Fatalf("second balance after = %d, want 500000", entries[0].BalanceAfterMicroCNY)
	}
}

func TestCreateAndListRechargeRequests(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "recharge-user-" + suffix + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Recharge User",
		PrimaryEmail:  &email,
		WorkspaceName: "Recharge Workspace",
		WorkspaceSlug: "recharge-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	first, err := repos.CreateRechargeRequest(ctx, CreateRechargeRequestInput{
		WorkspaceID:       result.Workspace.ID,
		RequestedByUserID: result.User.ID,
		AmountMicroCNY:    10_000_000,
		PaymentMethod:     "bank_transfer",
		Contact:           "ops@example.com",
		Note:              "first top up",
	})
	if err != nil {
		t.Fatalf("create first recharge request: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := repos.CreateRechargeRequest(ctx, CreateRechargeRequestInput{
		WorkspaceID:       result.Workspace.ID,
		RequestedByUserID: result.User.ID,
		AmountMicroCNY:    20_000_000,
		PaymentMethod:     "alipay",
		Contact:           "finance@example.com",
		Note:              "second top up",
	})
	if err != nil {
		t.Fatalf("create second recharge request: %v", err)
	}

	requests, err := repos.ListRechargeRequestsByWorkspace(ctx, result.Workspace.ID, 10)
	if err != nil {
		t.Fatalf("list recharge requests: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("requests len = %d, want 2", len(requests))
	}
	if requests[0].ID != second.ID || requests[1].ID != first.ID {
		t.Fatalf("requests order = [%s %s], want newest first [%s %s]", requests[0].ID, requests[1].ID, second.ID, first.ID)
	}
	if first.Status != domain.RechargeRequestStatusPending || first.Currency != "CNY" {
		t.Fatalf("first status/currency = %s/%s, want pending/CNY", first.Status, first.Currency)
	}
	if first.AmountMicroCNY != 10_000_000 || first.PaymentMethod != "bank_transfer" || first.Contact != "ops@example.com" || first.Note != "first top up" {
		t.Fatalf("first request = %+v", first)
	}
}
