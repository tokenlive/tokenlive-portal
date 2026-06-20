package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/database"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("PORTAL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PORTAL_TEST_DATABASE_DSN is not set")
	}

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	return db
}

func uniqueSuffix(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("read random bytes: %v", err)
	}

	return fmt.Sprintf("%s-%d-%s", strings.ToLower(name), time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
