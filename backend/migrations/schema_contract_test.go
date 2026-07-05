package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestModelPriceSchemaUsesPublishedSnapshotColumns(t *testing.T) {
	snapshot, err := os.ReadFile("../init.sql")
	if err != nil {
		t.Fatalf("read schema snapshot: %v", err)
	}
	migration, err := os.ReadFile("000002_model_catalog.sql")
	if err != nil {
		t.Fatalf("read model catalog migration: %v", err)
	}

	sql := string(snapshot) + "\n" + string(migration)
	forbidden := []string{
		"published_by_user_id",
		"fk_model_price_versions_published_by",
		"input_micro_cny_per_1m_tokens",
		"output_micro_cny_per_1m_tokens",
		"cache_read_micro_cny_per_1m_tokens",
	}
	for _, snippet := range forbidden {
		if strings.Contains(sql, snippet) {
			t.Fatalf("model price schema should not contain %q", snippet)
		}
	}
	for _, snippet := range []string{
		"input_price DECIMAL(18,9) NOT NULL",
		"output_price DECIMAL(18,9) NOT NULL",
		"cached_price DECIMAL(18,9) NULL",
		"cache_creation_price DECIMAL(18,9) NULL",
	} {
		if !strings.Contains(sql, snippet) {
			t.Fatalf("model price schema missing %q", snippet)
		}
	}
}
