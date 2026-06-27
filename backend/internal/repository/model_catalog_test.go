package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/datatypes"
)

func TestListPublicModelsFiltersAndLocaleFallback(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	now := time.Now().UTC().Truncate(time.Millisecond)

	publicModelID := "model-public-" + suffix
	privateModelID := "model-private-" + suffix
	pausedModelID := "model-paused-" + suffix

	models := []domain.ModelCatalog{
		{
			ModelID:     publicModelID,
			Slug:        "public-" + suffix,
			Status:      domain.ModelCatalogStatusAvailable,
			Visibility:  domain.ModelCatalogVisibilityPublic,
			Featured:    true,
			SortWeight:  99,
			PublishedAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ModelID:     privateModelID,
			Slug:        "private-" + suffix,
			Status:      domain.ModelCatalogStatusAvailable,
			Visibility:  domain.ModelCatalogVisibilityPrivate,
			PublishedAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ModelID:     pausedModelID,
			Slug:        "paused-" + suffix,
			Status:      domain.ModelCatalogStatusPaused,
			Visibility:  domain.ModelCatalogVisibilityPublic,
			PublishedAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create model catalogs: %v", err)
	}

	i18nRows := []domain.ModelCatalogI18n{
		{
			ModelID:          publicModelID,
			Locale:           DefaultLocale,
			DisplayName:      "中文公有名称",
			ShortDescription: "",
			UpdatedAt:        now,
		},
		{
			ModelID:          publicModelID,
			Locale:           FallbackLocale,
			DisplayName:      "English Public Name",
			ShortDescription: "English public short description",
			Tags:             datatypes.JSON([]byte(`["reasoning","coding"]`)),
			UpdatedAt:        now,
		},
		{
			ModelID:          privateModelID,
			Locale:           FallbackLocale,
			DisplayName:      "Private Name",
			ShortDescription: "Private short description",
			UpdatedAt:        now,
		},
		{
			ModelID:          pausedModelID,
			Locale:           FallbackLocale,
			DisplayName:      "Paused Name",
			ShortDescription: "Paused short description",
			UpdatedAt:        now,
		},
	}
	if err := db.Create(&i18nRows).Error; err != nil {
		t.Fatalf("create model i18n rows: %v", err)
	}

	modelsOut, err := repos.ListPublicModels(ctx, ModelCatalogListOptions{
		Locale: "zh-CN",
	})
	if err != nil {
		t.Fatalf("list public models: %v", err)
	}

	if len(modelsOut) != 1 {
		t.Fatalf("got %d models, want 1", len(modelsOut))
	}

	got := modelsOut[0]
	if got.ModelID != publicModelID {
		t.Fatalf("got model id %q, want %q", got.ModelID, publicModelID)
	}
	if got.DisplayName != "English Public Name" {
		t.Fatalf("got display name %q", got.DisplayName)
	}
	if got.ShortDescription != "English public short description" {
		t.Fatalf("got short description %q", got.ShortDescription)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "reasoning" || got.Tags[1] != "coding" {
		t.Fatalf("got tags %#v", got.Tags)
	}
}

func TestGetPublicModelBySlugIncludesCurrentPriceAndMetrics(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	knowledgeCutoff := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)

	modelID := "model-detail-" + suffix
	slug := "detail-" + suffix

	model := domain.ModelCatalog{
		ModelID:         modelID,
		Slug:            slug,
		Status:          domain.ModelCatalogStatusAvailable,
		Visibility:      domain.ModelCatalogVisibilityPublic,
		LogoURL:         "https://example.com/logo.png",
		KnowledgeCutoff: &knowledgeCutoff,
		Featured:        true,
		SortWeight:      10,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model catalog: %v", err)
	}

	longDescription := "Detailed description"
	i18nRows := []domain.ModelCatalogI18n{
		{
			ModelID:          modelID,
			Locale:           DefaultLocale,
			DisplayName:      "中文名",
			ShortDescription: "",
			LongDescription:  strPtr(""),
			SEOTitle:         "",
			SEODescription:   "",
			Tags:             datatypes.JSON([]byte(`["中文标签"]`)),
			UpdatedAt:        now,
		},
		{
			ModelID:          modelID,
			Locale:           FallbackLocale,
			DisplayName:      "English Name",
			ShortDescription: "English short description",
			LongDescription:  &longDescription,
			SEOTitle:         "English SEO Title",
			SEODescription:   "English SEO Description",
			UpdatedAt:        now,
		},
	}
	if err := db.Create(&i18nRows).Error; err != nil {
		t.Fatalf("create model i18n rows: %v", err)
	}

	expiredUntil := now.Add(-2 * time.Hour)
	currentFrom := now.Add(-30 * time.Minute)
	futureFrom := now.Add(2 * time.Hour)
	cacheReadPrice := 0.000300
	prices := []domain.ModelPriceVersion{
		{
			ID:            "prc-exp-" + suffix,
			ModelID:       modelID,
			Currency:      "CNY",
			InputPrice:    0.001000,
			OutputPrice:   0.002000,
			EffectiveFrom: now.Add(-4 * time.Hour),
			EffectiveUntil: &expiredUntil,
			Status:        domain.ModelPriceStatusActive,
			PublishedAt:   now.Add(-4 * time.Hour),
		},
		{
			ID:           "prc-cur-" + suffix,
			ModelID:      modelID,
			Currency:     "CNY",
			InputPrice:   0.003000,
			OutputPrice:  0.004000,
			CachedPrice:  &cacheReadPrice,
			EffectiveFrom: currentFrom,
			Status:       domain.ModelPriceStatusActive,
			PublishedAt:  currentFrom,
		},
		{
			ID:            "prc-fut-" + suffix,
			ModelID:       modelID,
			Currency:      "CNY",
			InputPrice:    0.009000,
			OutputPrice:   0.010000,
			EffectiveFrom: futureFrom,
			Status:        domain.ModelPriceStatusActive,
			PublishedAt:   futureFrom,
		},
	}
	if err := db.Create(&prices).Error; err != nil {
		t.Fatalf("create price rows: %v", err)
	}

	availability := 0.998
	successRate := 0.995
	lowAvailability := 0.95
	metrics := []domain.ModelServiceMetric{
		{
			ModelID:      modelID,
			Window:       "24h",
			Availability: &availability,
			TTFTP50MS:    int64Ptr(650),
			TTFTP95MS:    int64Ptr(1800),
			SuccessRate:  &successRate,
			SampleCount:  MinPublicMetricSampleCount,
			UpdatedAt:    now,
		},
		{
			ModelID:      modelID,
			Window:       "7d",
			Availability: &lowAvailability,
			SampleCount:  MinPublicMetricSampleCount - 1,
			UpdatedAt:    now,
		},
		{
			ModelID:      modelID,
			Window:       "30d",
			Availability: &availability,
			SampleCount:  MinPublicMetricSampleCount + 50,
			UpdatedAt:    now,
		},
	}
	if err := db.Create(&metrics).Error; err != nil {
		t.Fatalf("create metric rows: %v", err)
	}

	detail, err := repos.GetPublicModelBySlug(ctx, slug, DefaultLocale)
	if err != nil {
		t.Fatalf("get public model by slug: %v", err)
	}

	if detail.ModelID != modelID {
		t.Fatalf("got model id %q, want %q", detail.ModelID, modelID)
	}
	if detail.DisplayName != "中文名" {
		t.Fatalf("got display name %q", detail.DisplayName)
	}
	if detail.ShortDescription != "English short description" {
		t.Fatalf("got short description %q", detail.ShortDescription)
	}
	if detail.LongDescription != longDescription {
		t.Fatalf("got long description %q", detail.LongDescription)
	}
	if detail.SEOTitle != "English SEO Title" {
		t.Fatalf("got seo title %q", detail.SEOTitle)
	}
	if detail.SEODescription != "English SEO Description" {
		t.Fatalf("got seo description %q", detail.SEODescription)
	}
	if detail.Price == nil {
		t.Fatalf("expected active price")
	}
	if detail.Price.InputPrice != 0.003000 || detail.Price.OutputPrice != 0.004000 {
		t.Fatalf("got price %#v", detail.Price)
	}
	if detail.Price.CachedPrice == nil || *detail.Price.CachedPrice != cacheReadPrice {
		t.Fatalf("got cache read price %#v", detail.Price.CachedPrice)
	}
	if detail.Metrics == nil || detail.Metrics.Window != "24h" {
		t.Fatalf("got summary metrics %#v", detail.Metrics)
	}
	if len(detail.ServiceMetrics) != 1 || detail.ServiceMetrics[0].Window != "24h" {
		t.Fatalf("got service metrics %#v", detail.ServiceMetrics)
	}
	if detail.KnowledgeCutoff == nil || !detail.KnowledgeCutoff.Equal(knowledgeCutoff) {
		t.Fatalf("got knowledge cutoff %#v", detail.KnowledgeCutoff)
	}
}

func TestGetPublicModelBySlugReturnsNotFoundForPrivate(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	now := time.Now().UTC().Truncate(time.Millisecond)

	model := domain.ModelCatalog{
		ModelID:     "model-private-detail-" + suffix,
		Slug:        "private-detail-" + suffix,
		Status:      domain.ModelCatalogStatusAvailable,
		Visibility:  domain.ModelCatalogVisibilityPrivate,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create private model catalog: %v", err)
	}

	_, err := repos.GetPublicModelBySlug(ctx, model.Slug, DefaultLocale)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("got err %v, want ErrModelNotFound", err)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func strPtr(v string) *string {
	return &v
}
