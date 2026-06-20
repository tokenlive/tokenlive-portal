package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestResolveCurrentWorkspacePrefersOwnedWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "workspace-owner-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Owner",
		PrimaryEmail:  &email,
		WorkspaceName: "Owned Workspace",
		WorkspaceSlug: "owned-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	otherEmail := "workspace-other-" + suffix + "@example.com"
	otherResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Other",
		PrimaryEmail:  &otherEmail,
		WorkspaceName: "Other Workspace",
		WorkspaceSlug: "other-" + suffix,
	})
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	now := time.Now().UTC()
	member := domain.WorkspaceMember{
		WorkspaceID: otherResult.Workspace.ID,
		UserID:      userResult.User.ID,
		Role:        domain.MemberRoleDeveloper,
		Status:      domain.MemberStatusActive,
		JoinedAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create extra membership: %v", err)
	}

	current, err := repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if err != nil {
		t.Fatalf("resolve current workspace: %v", err)
	}
	if current.Workspace.ID != userResult.Workspace.ID {
		t.Fatalf("workspace id = %s, want owned %s", current.Workspace.ID, userResult.Workspace.ID)
	}
	if current.Member.WorkspaceID != userResult.Workspace.ID {
		t.Fatalf("member workspace id = %s, want %s", current.Member.WorkspaceID, userResult.Workspace.ID)
	}
	if current.Role != domain.MemberRoleOwner {
		t.Fatalf("role = %s, want owner", current.Role)
	}
}

func TestResolveCurrentWorkspaceFallsBackToOldestMembership(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "member-only-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Member",
		PrimaryEmail:  &email,
		WorkspaceName: "Member Owned",
		WorkspaceSlug: "member-owned-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&domain.Workspace{}).
		Where("id = ?", userResult.Workspace.ID).
		Update("status", domain.WorkspaceStatusDeleting).Error; err != nil {
		t.Fatalf("deactivate owned workspace: %v", err)
	}

	hostEmail := "host-" + suffix + "@example.com"
	hostResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Host",
		PrimaryEmail:  &hostEmail,
		WorkspaceName: "Host Workspace",
		WorkspaceSlug: "host-" + suffix,
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	newerEmail := "newer-host-" + suffix + "@example.com"
	newerHostResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Newer Host",
		PrimaryEmail:  &newerEmail,
		WorkspaceName: "Newer Host Workspace",
		WorkspaceSlug: "newer-host-" + suffix,
	})
	if err != nil {
		t.Fatalf("create newer host: %v", err)
	}

	oldJoinedAt := time.Now().UTC().Add(-2 * time.Hour)
	newJoinedAt := time.Now().UTC().Add(-time.Hour)
	members := []domain.WorkspaceMember{
		{
			WorkspaceID: newerHostResult.Workspace.ID,
			UserID:      userResult.User.ID,
			Role:        domain.MemberRoleDeveloper,
			Status:      domain.MemberStatusActive,
			JoinedAt:    &newJoinedAt,
			CreatedAt:   newJoinedAt,
			UpdatedAt:   newJoinedAt,
		},
		{
			WorkspaceID: hostResult.Workspace.ID,
			UserID:      userResult.User.ID,
			Role:        domain.MemberRoleBilling,
			Status:      domain.MemberStatusActive,
			JoinedAt:    &oldJoinedAt,
			CreatedAt:   oldJoinedAt,
			UpdatedAt:   oldJoinedAt,
		},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("create fallback memberships: %v", err)
	}

	current, err := repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if err != nil {
		t.Fatalf("resolve current workspace: %v", err)
	}
	if current.Workspace.ID != hostResult.Workspace.ID {
		t.Fatalf("workspace id = %s, want fallback %s", current.Workspace.ID, hostResult.Workspace.ID)
	}
	if current.Role != domain.MemberRoleBilling {
		t.Fatalf("role = %s, want billing", current.Role)
	}
}

func TestResolveCurrentWorkspaceReturnsZeroBalanceWhenBalanceMissing(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "workspace-no-balance-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "No Balance",
		PrimaryEmail:  &email,
		WorkspaceName: "No Balance Workspace",
		WorkspaceSlug: "no-balance-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Where("workspace_id = ?", userResult.Workspace.ID).Delete(&domain.WorkspaceBalance{}).Error; err != nil {
		t.Fatalf("delete balance: %v", err)
	}

	current, err := repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if err != nil {
		t.Fatalf("resolve current workspace: %v", err)
	}
	if current.Balance.WorkspaceID != userResult.Workspace.ID {
		t.Fatalf("balance workspace id = %s, want %s", current.Balance.WorkspaceID, userResult.Workspace.ID)
	}
	if current.Balance.AvailableMicroCNY != 0 || current.Balance.FrozenMicroCNY != 0 {
		t.Fatalf("balance = %+v, want zero balance", current.Balance)
	}
}

func TestResolveCurrentWorkspaceReturnsNotFoundWithoutActiveMembership(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "workspace-removed-member-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Removed Member",
		PrimaryEmail:  &email,
		WorkspaceName: "Removed Member Workspace",
		WorkspaceSlug: "removed-member-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&domain.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", userResult.Workspace.ID, userResult.User.ID).
		Update("status", domain.MemberStatusRemoved).Error; err != nil {
		t.Fatalf("remove membership: %v", err)
	}

	_, err = repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("err = %v, want ErrWorkspaceNotFound", err)
	}
}
