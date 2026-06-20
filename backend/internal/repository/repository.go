package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"
)

type Repositories struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repositories {
	return &Repositories{db: db}
}

func (r *Repositories) DB() *gorm.DB {
	return r.db
}

func (r *Repositories) withTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func newID(prefix string) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	return prefix + hex.EncodeToString(b[:]), nil
}
