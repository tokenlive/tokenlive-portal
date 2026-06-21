package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDuplicateLedgerConflict = errors.New("duplicate ledger idempotency key with conflicting payload")
	ErrInsufficientBalance     = errors.New("insufficient balance")
)

type CreateLedgerInput struct {
	WorkspaceID              string
	Type                     domain.LedgerType
	Direction                domain.LedgerDirection
	AmountMicroCNY           int64
	Currency                 string
	IdempotencyKey           string
	RequestID                *string
	APIKeyID                 *string
	APIKeyNameSnapshot       string
	ModelID                  string
	ModelDisplayNameSnapshot string
	PriceVersionID           string
	UnitPriceSnapshot        datatypes.JSON
	Metadata                 datatypes.JSON
}

func (r *Repositories) CreateLedgerEntry(ctx context.Context, input CreateLedgerInput) (domain.LedgerEntry, error) {
	var output domain.LedgerEntry
	now := time.Now().UTC()

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		entry, err := createLedgerEntryInTx(tx, input, now)
		if err != nil {
			return err
		}
		output = entry
		return nil
	})
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	return output, nil
}

func grantTrialCreditInTx(tx *gorm.DB, workspaceID string, now time.Time, input TrialCreditInput) error {
	if input.AmountMicroCNY == 0 {
		return nil
	}
	if input.AmountMicroCNY < 0 {
		return fmt.Errorf("trial credit amount must be greater than or equal to zero")
	}
	if input.TTLDays <= 0 {
		return fmt.Errorf("trial credit ttl days must be greater than zero")
	}

	var workspace domain.Workspace
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", workspaceID).
		First(&workspace).Error; err != nil {
		return fmt.Errorf("lock trial workspace: %w", err)
	}
	if workspace.TrialGrantedAt != nil {
		return nil
	}

	expiresAt := now.AddDate(0, 0, input.TTLDays)
	source := input.Source
	if source == "" {
		source = "email_registration"
	}
	metadata, err := json.Marshal(map[string]any{
		"source":           source,
		"trial_expires_at": expiresAt.Format(time.RFC3339Nano),
		"trial_ttl_days":   input.TTLDays,
	})
	if err != nil {
		return fmt.Errorf("marshal trial metadata: %w", err)
	}

	entryInput := CreateLedgerInput{
		WorkspaceID:    workspaceID,
		Type:           domain.LedgerTypeTrialGrant,
		Direction:      domain.LedgerDirectionCredit,
		AmountMicroCNY: input.AmountMicroCNY,
		Currency:       "CNY",
		IdempotencyKey: "trial-grant:" + workspaceID,
		Metadata:       datatypes.JSON(metadata),
	}
	if _, err := createLedgerEntryInTx(tx, entryInput, now); err != nil {
		return err
	}

	if err := tx.Model(&domain.Workspace{}).
		Where("id = ? AND trial_granted_at IS NULL", workspaceID).
		Updates(map[string]any{
			"trial_granted_at": now,
			"updated_at":       now,
		}).Error; err != nil {
		return fmt.Errorf("mark trial granted: %w", err)
	}

	return nil
}

func createLedgerEntryInTx(tx *gorm.DB, input CreateLedgerInput, now time.Time) (domain.LedgerEntry, error) {
	existing, found, err := findLedgerEntryForUpdate(tx, input.WorkspaceID, input.IdempotencyKey)
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	if found {
		if !sameLedgerPayload(existing, input) {
			return domain.LedgerEntry{}, ErrDuplicateLedgerConflict
		}
		return existing, nil
	}

	var balance domain.WorkspaceBalance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ?", input.WorkspaceID).
		First(&balance).Error; err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("lock workspace balance: %w", err)
	}

	nextAvailable := balance.AvailableMicroCNY
	switch input.Direction {
	case domain.LedgerDirectionCredit:
		nextAvailable += input.AmountMicroCNY
	case domain.LedgerDirectionDebit:
		if nextAvailable < input.AmountMicroCNY {
			return domain.LedgerEntry{}, ErrInsufficientBalance
		}
		nextAvailable -= input.AmountMicroCNY
	default:
		return domain.LedgerEntry{}, fmt.Errorf("unsupported ledger direction: %s", input.Direction)
	}

	id, err := newID("led_")
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	entry := domain.LedgerEntry{
		ID:                       id,
		WorkspaceID:              input.WorkspaceID,
		Type:                     input.Type,
		Direction:                input.Direction,
		AmountMicroCNY:           input.AmountMicroCNY,
		BalanceAfterMicroCNY:     nextAvailable,
		Currency:                 input.Currency,
		IdempotencyKey:           input.IdempotencyKey,
		RequestID:                input.RequestID,
		APIKeyID:                 input.APIKeyID,
		APIKeyNameSnapshot:       input.APIKeyNameSnapshot,
		ModelID:                  input.ModelID,
		ModelDisplayNameSnapshot: input.ModelDisplayNameSnapshot,
		PriceVersionID:           input.PriceVersionID,
		UnitPriceSnapshot:        input.UnitPriceSnapshot,
		Metadata:                 input.Metadata,
		CreatedAt:                now,
	}
	if err := tx.Create(&entry).Error; err != nil {
		if isMySQLDuplicateError(err) {
			existing, found, lookupErr := findLedgerEntryForUpdate(tx, input.WorkspaceID, input.IdempotencyKey)
			if lookupErr != nil {
				return domain.LedgerEntry{}, lookupErr
			}
			if found && sameLedgerPayload(existing, input) {
				return existing, nil
			}
			return domain.LedgerEntry{}, ErrDuplicateLedgerConflict
		}
		return domain.LedgerEntry{}, fmt.Errorf("create ledger entry: %w", err)
	}

	if err := tx.Model(&domain.WorkspaceBalance{}).
		Where("workspace_id = ?", input.WorkspaceID).
		Updates(map[string]any{
			"available_micro_cny": nextAvailable,
			"version":             balance.Version + 1,
			"updated_at":          now,
		}).Error; err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("update workspace balance: %w", err)
	}

	return entry, nil
}

func findLedgerEntryForUpdate(tx *gorm.DB, workspaceID string, idempotencyKey string) (domain.LedgerEntry, bool, error) {
	var existing domain.LedgerEntry
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND idempotency_key = ?", workspaceID, idempotencyKey).
		First(&existing).Error
	if err == nil {
		return existing, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.LedgerEntry{}, false, nil
	}
	return domain.LedgerEntry{}, false, fmt.Errorf("lookup existing ledger: %w", err)
}

func sameLedgerPayload(existing domain.LedgerEntry, input CreateLedgerInput) bool {
	return existing.Type == input.Type &&
		existing.Direction == input.Direction &&
		existing.AmountMicroCNY == input.AmountMicroCNY &&
		existing.Currency == input.Currency &&
		pointerValue(existing.RequestID) == pointerValue(input.RequestID) &&
		pointerValue(existing.APIKeyID) == pointerValue(input.APIKeyID) &&
		existing.APIKeyNameSnapshot == input.APIKeyNameSnapshot &&
		existing.ModelID == input.ModelID &&
		existing.ModelDisplayNameSnapshot == input.ModelDisplayNameSnapshot &&
		existing.PriceVersionID == input.PriceVersionID &&
		string(existing.UnitPriceSnapshot) == string(input.UnitPriceSnapshot) &&
		string(existing.Metadata) == string(input.Metadata)
}

func isMySQLDuplicateError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func pointerValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
