package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrModelNotFound = errors.New("model not found")

const (
	DefaultLocale              = "zh-CN"
	FallbackLocale             = "en"
	MinPublicMetricSampleCount = int64(100)
)

type ModelCatalogListOptions struct {
	Locale   string
	Featured *bool
	Limit    int
	Offset   int
}

type PublicModel struct {
	ModelID          string
	Slug             string
	Status           domain.ModelCatalogStatus
	DisplayName      string
	ShortDescription string
	Tags             []string
	ContextLength    *int64
	InputModalities  []string
	OutputModalities []string
	Capabilities     []string
	Featured         bool
	KnowledgeCutoff  *time.Time
	LogoURL          string
	Price            *PublicModelPrice
	Metrics          *PublicModelMetric
	PublishedAt      *time.Time
	SortWeight       int64
}

type PublicModelDetail struct {
	PublicModel
	LongDescription string
	SEOTitle        string
	SEODescription  string
	ServiceMetrics  []PublicModelMetric
}

type PublicModelPrice struct {
	Currency           string
	InputPrice         float64
	OutputPrice        float64
	CachedPrice        *float64
	CacheCreationPrice *float64
}

type PublicModelMetric struct {
	Window        string
	Availability  *float64
	TTFTP50MS     *int64
	TTFTP95MS     *int64
	ResponseSpeed *float64
	SuccessRate   *float64
	SampleCount   int64
	UpdatedAt     time.Time
}

type PublishModelCatalogInput struct {
	Catalog domain.ModelCatalog
	I18n    []domain.ModelCatalogI18n
	Prices  []domain.ModelPriceVersion
}

func (r *Repositories) PublishModelCatalog(ctx context.Context, input PublishModelCatalogInput) error {
	return r.withTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "model_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"slug",
				"status",
				"visibility",
				"logo_url",
				"context_length",
				"knowledge_cutoff",
				"input_modalities",
				"output_modalities",
				"capabilities",
				"featured",
				"sort_weight",
				"published_at",
				"updated_at",
			}),
		}).Create(&input.Catalog).Error; err != nil {
			return fmt.Errorf("upsert model catalog: %w", err)
		}

		if len(input.I18n) > 0 {
			for i := range input.I18n {
				input.I18n[i].ModelID = input.Catalog.ModelID
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "model_id"}, {Name: "locale"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"display_name",
					"short_description",
					"long_description",
					"seo_title",
					"seo_description",
					"tags",
					"updated_at",
				}),
			}).Create(&input.I18n).Error; err != nil {
				return fmt.Errorf("upsert model catalog i18n: %w", err)
			}
		}

		if len(input.Prices) > 0 {
			for i := range input.Prices {
				input.Prices[i].ModelID = input.Catalog.ModelID
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "model_id"}, {Name: "effective_from"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"id",
					"currency",
					"input_price",
					"output_price",
					"cached_price",
					"cache_creation_price",
					"effective_until",
					"status",
					"published_at",
				}),
			}).Create(&input.Prices).Error; err != nil {
				return fmt.Errorf("upsert model price versions: %w", err)
			}
		}

		return nil
	})
}

func (r *Repositories) ListPublicModels(ctx context.Context, opts ModelCatalogListOptions) ([]PublicModel, error) {
	locale := normalizeCatalogLocale(opts.Locale)
	limit := clampCatalogLimit(opts.Limit)
	offset := clampCatalogOffset(opts.Offset)

	rows := make([]publicModelRow, 0, limit)
	query := r.db.WithContext(ctx).
		Table("model_catalogs AS m").
		Select(`
			m.model_id,
			m.slug,
			m.status,
			m.logo_url,
			m.context_length,
			m.knowledge_cutoff,
			m.input_modalities,
			m.output_modalities,
			m.capabilities,
			m.featured,
			m.sort_weight,
			m.published_at,
			COALESCE(NULLIF(req.display_name, ''), fb.display_name, '') AS display_name,
			COALESCE(NULLIF(req.short_description, ''), fb.short_description, '') AS short_description,
			COALESCE(req.tags, fb.tags) AS tags
		`).
		Joins("LEFT JOIN model_catalog_i18n AS req ON req.model_id = m.model_id AND req.locale = ?", locale).
		Joins("LEFT JOIN model_catalog_i18n AS fb ON fb.model_id = m.model_id AND fb.locale = ?", FallbackLocale).
		Where("m.visibility = ? AND m.status = ? AND m.published_at IS NOT NULL",
			domain.ModelCatalogVisibilityPublic,
			domain.ModelCatalogStatusAvailable,
		).
		Order("m.featured DESC").
		Order("m.sort_weight DESC").
		Order("m.published_at DESC").
		Order("m.model_id ASC").
		Limit(limit).
		Offset(offset)
	if opts.Featured != nil {
		query = query.Where("m.featured = ?", *opts.Featured)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list public models: %w", err)
	}

	models, err := buildPublicModels(rows)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return models, nil
	}

	modelIDs := collectModelIDs(models)
	priceByModelID, err := r.loadCurrentPrices(ctx, modelIDs, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	metricsByModelID, err := r.loadDisplayMetrics(ctx, modelIDs)
	if err != nil {
		return nil, err
	}

	for i := range models {
		models[i].Price = priceByModelID[models[i].ModelID]
		models[i].Metrics = pickPrimaryMetric(metricsByModelID[models[i].ModelID])
	}

	return models, nil
}

func (r *Repositories) GetPublicModelBySlug(ctx context.Context, slug string, locale string) (PublicModelDetail, error) {
	row := publicModelDetailRow{}
	err := r.db.WithContext(ctx).
		Table("model_catalogs AS m").
		Select(`
			m.model_id,
			m.slug,
			m.status,
			m.logo_url,
			m.context_length,
			m.knowledge_cutoff,
			m.input_modalities,
			m.output_modalities,
			m.capabilities,
			m.featured,
			m.sort_weight,
			m.published_at,
			COALESCE(NULLIF(req.display_name, ''), fb.display_name, '') AS display_name,
			COALESCE(NULLIF(req.short_description, ''), fb.short_description, '') AS short_description,
			COALESCE(NULLIF(req.long_description, ''), fb.long_description, '') AS long_description,
			COALESCE(NULLIF(req.seo_title, ''), fb.seo_title, '') AS seo_title,
			COALESCE(NULLIF(req.seo_description, ''), fb.seo_description, '') AS seo_description,
			COALESCE(req.tags, fb.tags) AS tags
		`).
		Joins("LEFT JOIN model_catalog_i18n AS req ON req.model_id = m.model_id AND req.locale = ?", normalizeCatalogLocale(locale)).
		Joins("LEFT JOIN model_catalog_i18n AS fb ON fb.model_id = m.model_id AND fb.locale = ?", FallbackLocale).
		Where("m.slug = ? AND m.visibility = ? AND m.status = ? AND m.published_at IS NOT NULL",
			slug,
			domain.ModelCatalogVisibilityPublic,
			domain.ModelCatalogStatusAvailable,
		).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return PublicModelDetail{}, fmt.Errorf("get public model by slug: %w", err)
	}
	if row.ModelID == "" {
		return PublicModelDetail{}, ErrModelNotFound
	}

	models, err := buildPublicModels([]publicModelRow{row.publicModelRow})
	if err != nil {
		return PublicModelDetail{}, err
	}
	detail := PublicModelDetail{
		PublicModel:     models[0],
		LongDescription: row.LongDescription,
		SEOTitle:        row.SEOTitle,
		SEODescription:  row.SEODescription,
		ServiceMetrics:  nil,
	}

	priceByModelID, err := r.loadCurrentPrices(ctx, []string{detail.ModelID}, time.Now().UTC())
	if err != nil {
		return PublicModelDetail{}, err
	}
	metricsByModelID, err := r.loadDisplayMetrics(ctx, []string{detail.ModelID})
	if err != nil {
		return PublicModelDetail{}, err
	}

	detail.Price = priceByModelID[detail.ModelID]
	detail.ServiceMetrics = metricsByModelID[detail.ModelID]
	detail.Metrics = pickPrimaryMetric(detail.ServiceMetrics)

	return detail, nil
}

type publicModelRow struct {
	ModelID          string
	Slug             string
	Status           domain.ModelCatalogStatus
	LogoURL          string
	ContextLength    *int64
	KnowledgeCutoff  *time.Time
	InputModalities  datatypes.JSON
	OutputModalities datatypes.JSON
	Capabilities     datatypes.JSON
	Featured         bool
	SortWeight       int64
	PublishedAt      *time.Time
	DisplayName      string
	ShortDescription string
	Tags             datatypes.JSON
}

type publicModelDetailRow struct {
	publicModelRow
	LongDescription string
	SEOTitle        string
	SEODescription  string
}

type publicModelPriceRow struct {
	ModelID            string
	Currency           string
	InputPrice         float64
	OutputPrice        float64
	CachedPrice        *float64
	CacheCreationPrice *float64
}

type publicModelMetricRow struct {
	ModelID       string
	Window        string
	Availability  *float64
	TTFTP50MS     *int64
	TTFTP95MS     *int64
	ResponseSpeed *float64
	SuccessRate   *float64
	SampleCount   int64
	UpdatedAt     time.Time
}

func normalizeCatalogLocale(locale string) string {
	switch locale {
	case DefaultLocale, FallbackLocale:
		return locale
	default:
		return DefaultLocale
	}
}

func clampCatalogLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func clampCatalogOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func buildPublicModels(rows []publicModelRow) ([]PublicModel, error) {
	models := make([]PublicModel, 0, len(rows))
	for _, row := range rows {
		tags, err := decodeStringSlice(row.Tags)
		if err != nil {
			return nil, fmt.Errorf("decode model %s tags: %w", row.ModelID, err)
		}
		inputModalities, err := decodeStringSlice(row.InputModalities)
		if err != nil {
			return nil, fmt.Errorf("decode model %s input modalities: %w", row.ModelID, err)
		}
		outputModalities, err := decodeStringSlice(row.OutputModalities)
		if err != nil {
			return nil, fmt.Errorf("decode model %s output modalities: %w", row.ModelID, err)
		}
		capabilities, err := decodeStringSlice(row.Capabilities)
		if err != nil {
			return nil, fmt.Errorf("decode model %s capabilities: %w", row.ModelID, err)
		}

		models = append(models, PublicModel{
			ModelID:          row.ModelID,
			Slug:             row.Slug,
			Status:           row.Status,
			DisplayName:      row.DisplayName,
			ShortDescription: row.ShortDescription,
			Tags:             tags,
			ContextLength:    row.ContextLength,
			InputModalities:  inputModalities,
			OutputModalities: outputModalities,
			Capabilities:     capabilities,
			Featured:         row.Featured,
			KnowledgeCutoff:  row.KnowledgeCutoff,
			LogoURL:          row.LogoURL,
			PublishedAt:      row.PublishedAt,
			SortWeight:       row.SortWeight,
		})
	}

	return models, nil
}

func decodeStringSlice(raw datatypes.JSON) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func collectModelIDs(models []PublicModel) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ModelID)
	}
	return ids
}

func (r *Repositories) loadCurrentPrices(ctx context.Context, modelIDs []string, now time.Time) (map[string]*PublicModelPrice, error) {
	if len(modelIDs) == 0 {
		return map[string]*PublicModelPrice{}, nil
	}

	subquery := r.db.WithContext(ctx).
		Table("model_price_versions").
		Select("model_id, MAX(effective_from) AS effective_from").
		Where("model_id IN ?", modelIDs).
		Where("status = ?", domain.ModelPriceStatusActive).
		Where("effective_from <= ?", now).
		Where("effective_until IS NULL OR effective_until > ?", now).
		Group("model_id")

	rows := make([]publicModelPriceRow, 0, len(modelIDs))
	if err := r.db.WithContext(ctx).
		Table("model_price_versions AS p").
		Select(`
			p.model_id,
			p.currency,
			p.input_price,
			p.output_price,
			p.cached_price,
			p.cache_creation_price
		`).
		Joins("JOIN (?) AS current ON current.model_id = p.model_id AND current.effective_from = p.effective_from", subquery).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load current public prices: %w", err)
	}

	priceByModelID := make(map[string]*PublicModelPrice, len(rows))
	for _, row := range rows {
		price := PublicModelPrice{
			Currency:           row.Currency,
			InputPrice:         row.InputPrice,
			OutputPrice:        row.OutputPrice,
			CachedPrice:        row.CachedPrice,
			CacheCreationPrice: row.CacheCreationPrice,
		}
		priceByModelID[row.ModelID] = &price
	}

	return priceByModelID, nil
}

func (r *Repositories) loadDisplayMetrics(ctx context.Context, modelIDs []string) (map[string][]PublicModelMetric, error) {
	if len(modelIDs) == 0 {
		return map[string][]PublicModelMetric{}, nil
	}

	rows := make([]publicModelMetricRow, 0)
	if err := r.db.WithContext(ctx).
		Table("model_service_metrics").
		Select("model_id, `window`, availability, ttft_p50_ms, ttft_p95_ms, response_speed, success_rate, sample_count, updated_at").
		Where("model_id IN ?", modelIDs).
		Where("`window` IN ?", []string{"24h", "7d"}).
		Where("sample_count >= ?", MinPublicMetricSampleCount).
		Order("CASE `window` WHEN '24h' THEN 0 WHEN '7d' THEN 1 ELSE 2 END ASC").
		Order("updated_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load public metrics: %w", err)
	}

	metricsByModelID := make(map[string][]PublicModelMetric, len(modelIDs))
	for _, row := range rows {
		metricsByModelID[row.ModelID] = append(metricsByModelID[row.ModelID], PublicModelMetric{
			Window:        row.Window,
			Availability:  row.Availability,
			TTFTP50MS:     row.TTFTP50MS,
			TTFTP95MS:     row.TTFTP95MS,
			ResponseSpeed: row.ResponseSpeed,
			SuccessRate:   row.SuccessRate,
			SampleCount:   row.SampleCount,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	for modelID := range metricsByModelID {
		slices.SortStableFunc(metricsByModelID[modelID], comparePublicMetrics)
	}

	return metricsByModelID, nil
}

func pickPrimaryMetric(metrics []PublicModelMetric) *PublicModelMetric {
	if len(metrics) == 0 {
		return nil
	}

	primary := metrics[0]
	return &primary
}

func comparePublicMetrics(a PublicModelMetric, b PublicModelMetric) int {
	if rankDiff := metricWindowRank(a.Window) - metricWindowRank(b.Window); rankDiff != 0 {
		return rankDiff
	}
	if a.UpdatedAt.After(b.UpdatedAt) {
		return -1
	}
	if a.UpdatedAt.Before(b.UpdatedAt) {
		return 1
	}
	if a.Window < b.Window {
		return -1
	}
	if a.Window > b.Window {
		return 1
	}
	return 0
}

func metricWindowRank(window string) int {
	switch window {
	case "24h":
		return 0
	case "7d":
		return 1
	default:
		return 2
	}
}
