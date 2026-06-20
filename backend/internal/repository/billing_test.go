package repository

import (
	"context"
	"errors"
	"testing"

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
