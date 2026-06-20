package database

import (
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("database dsn is required")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}
