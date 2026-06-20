package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestCreateUserWithDefaultWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	suffix := uniqueSuffix(t)

	email := "test-user-" + suffix + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(context.Background(), CreateUserWithWorkspaceInput{
		DisplayName:   "Test User",
		PrimaryEmail:  &email,
		WorkspaceName: "Test Workspace",
		WorkspaceSlug: "test-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	if result.User.ID == "" || result.Workspace.ID == "" {
		t.Fatalf("expected user and workspace ids")
	}

	var member domain.WorkspaceMember
	if err := db.Where("workspace_id = ? AND user_id = ?", result.Workspace.ID, result.User.ID).First(&member).Error; err != nil {
		t.Fatalf("find owner member: %v", err)
	}

	if member.Role != domain.MemberRoleOwner {
		t.Fatalf("got role %s", member.Role)
	}
}

func TestCreateWorkspaceEnforcesSequentialSelfCreatedLimit(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "workspace-limit-" + suffix + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Workspace Limit User",
		PrimaryEmail:  &email,
		WorkspaceName: "Default Workspace",
		WorkspaceSlug: "workspace-limit-default-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	for i := 1; i <= 2; i++ {
		_, err := repos.CreateWorkspace(ctx, CreateWorkspaceInput{
			Name:            fmt.Sprintf("Extra Workspace %d", i),
			Slug:            fmt.Sprintf("workspace-limit-extra-%s-%d", suffix, i),
			OwnerUserID:     result.User.ID,
			CreatedByUserID: result.User.ID,
		})
		if err != nil {
			t.Fatalf("create workspace %d: %v", i, err)
		}
	}

	_, err = repos.CreateWorkspace(ctx, CreateWorkspaceInput{
		Name:            "Extra Workspace 3",
		Slug:            "workspace-limit-extra-" + suffix + "-3",
		OwnerUserID:     result.User.ID,
		CreatedByUserID: result.User.ID,
	})
	if !errors.Is(err, ErrWorkspaceLimitExceeded) {
		t.Fatalf("got err %v, want ErrWorkspaceLimitExceeded", err)
	}
}
