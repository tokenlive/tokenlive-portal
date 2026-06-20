package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

type PublicModelReader interface {
	ListPublicModels(ctx context.Context, opts repository.ModelCatalogListOptions) ([]repository.PublicModel, error)
	GetPublicModelBySlug(ctx context.Context, slug string, locale string) (repository.PublicModelDetail, error)
}

type PublicModelHandler struct {
	reader PublicModelReader
}

func RegisterPublicModelRoutes(mux *http.ServeMux, reader PublicModelReader) {
	handler := PublicModelHandler{reader: reader}
	mux.HandleFunc("GET /api/public/models", handler.List)
	mux.HandleFunc("GET /api/public/models/{slug}", handler.Detail)
}

func (h PublicModelHandler) List(w http.ResponseWriter, r *http.Request) {
	opts, err := parsePublicModelListOptions(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrModelInvalidQuery)
		return
	}

	models, err := h.reader.ListPublicModels(r.Context(), opts)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	writeJSON(w, http.StatusOK, publicModelListResponse{
		Data: marshalPublicModels(models),
		Pagination: publicModelPagination{
			Limit:  normalizePublicModelLimit(opts.Limit),
			Offset: normalizePublicModelOffset(opts.Offset),
		},
	})
}

func (h PublicModelHandler) Detail(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	model, err := h.reader.GetPublicModelBySlug(r.Context(), r.PathValue("slug"), locale)
	if err != nil {
		if errors.Is(err, repository.ErrModelNotFound) {
			WriteError(w, RequestIDFromContext(r.Context()), ErrModelNotFound)
			return
		}
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	writeJSON(w, http.StatusOK, publicModelDetailResponse{
		Data: marshalPublicModelDetail(model),
	})
}

type publicModelListResponse struct {
	Data       []publicModelListPayload `json:"data"`
	Pagination publicModelPagination    `json:"pagination"`
}

type publicModelDetailResponse struct {
	Data publicModelDetailPayload `json:"data"`
}

type publicModelPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type publicModelListPayload struct {
	ModelID          string                    `json:"model_id"`
	Slug             string                    `json:"slug"`
	Status           string                    `json:"status"`
	DisplayName      string                    `json:"display_name"`
	ShortDescription string                    `json:"short_description"`
	Tags             []string                  `json:"tags,omitempty"`
	ContextLength    *int64                    `json:"context_length,omitempty"`
	InputModalities  []string                  `json:"input_modalities,omitempty"`
	OutputModalities []string                  `json:"output_modalities,omitempty"`
	Capabilities     []string                  `json:"capabilities,omitempty"`
	Featured         bool                      `json:"featured"`
	Price            *publicModelPricePayload  `json:"price,omitempty"`
	Metrics          *publicModelMetricPayload `json:"metrics,omitempty"`
}

type publicModelDetailPayload struct {
	publicModelListPayload
	KnowledgeCutoff *time.Time                 `json:"knowledge_cutoff,omitempty"`
	LogoURL         string                     `json:"logo_url,omitempty"`
	LongDescription string                     `json:"long_description"`
	SEOTitle        string                     `json:"seo_title"`
	SEODescription  string                     `json:"seo_description"`
	ServiceMetrics  []publicModelMetricPayload `json:"service_metrics,omitempty"`
}

type publicModelPricePayload struct {
	Currency                     string `json:"currency"`
	InputMicroCNYPer1MTokens     int64  `json:"input_micro_cny_per_1m_tokens"`
	OutputMicroCNYPer1MTokens    int64  `json:"output_micro_cny_per_1m_tokens"`
	CacheReadMicroCNYPer1MTokens *int64 `json:"cache_read_micro_cny_per_1m_tokens,omitempty"`
}

type publicModelMetricPayload struct {
	Window        string    `json:"window"`
	Availability  *float64  `json:"availability,omitempty"`
	TTFTP50MS     *int64    `json:"ttft_p50_ms,omitempty"`
	TTFTP95MS     *int64    `json:"ttft_p95_ms,omitempty"`
	ResponseSpeed *float64  `json:"response_speed,omitempty"`
	SuccessRate   *float64  `json:"success_rate,omitempty"`
	SampleCount   int64     `json:"sample_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func parsePublicModelListOptions(r *http.Request) (repository.ModelCatalogListOptions, error) {
	query := r.URL.Query()
	opts := repository.ModelCatalogListOptions{
		Locale: query.Get("locale"),
	}

	if featuredRaw := query.Get("featured"); featuredRaw != "" {
		featured, err := strconv.ParseBool(featuredRaw)
		if err != nil {
			return repository.ModelCatalogListOptions{}, err
		}
		opts.Featured = &featured
	}

	if limitRaw := query.Get("limit"); limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)
		if err != nil {
			return repository.ModelCatalogListOptions{}, err
		}
		opts.Limit = limit
	}

	if offsetRaw := query.Get("offset"); offsetRaw != "" {
		offset, err := strconv.Atoi(offsetRaw)
		if err != nil {
			return repository.ModelCatalogListOptions{}, err
		}
		opts.Offset = offset
	}

	return opts, nil
}

func normalizePublicModelLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func normalizePublicModelOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func marshalPublicModels(models []repository.PublicModel) []publicModelListPayload {
	payloads := make([]publicModelListPayload, 0, len(models))
	for _, model := range models {
		payloads = append(payloads, marshalPublicModelList(model))
	}
	return payloads
}

func marshalPublicModelDetail(model repository.PublicModelDetail) publicModelDetailPayload {
	metrics := make([]publicModelMetricPayload, 0, len(model.ServiceMetrics))
	for _, metric := range model.ServiceMetrics {
		metrics = append(metrics, marshalPublicModelMetric(metric))
	}

	return publicModelDetailPayload{
		publicModelListPayload: marshalPublicModelList(model.PublicModel),
		KnowledgeCutoff:        model.KnowledgeCutoff,
		LogoURL:                model.LogoURL,
		LongDescription:        model.LongDescription,
		SEOTitle:               model.SEOTitle,
		SEODescription:         model.SEODescription,
		ServiceMetrics:         metrics,
	}
}

func marshalPublicModelList(model repository.PublicModel) publicModelListPayload {
	payload := publicModelListPayload{
		ModelID:          model.ModelID,
		Slug:             model.Slug,
		Status:           string(model.Status),
		DisplayName:      model.DisplayName,
		ShortDescription: model.ShortDescription,
		Tags:             model.Tags,
		ContextLength:    model.ContextLength,
		InputModalities:  model.InputModalities,
		OutputModalities: model.OutputModalities,
		Capabilities:     model.Capabilities,
		Featured:         model.Featured,
	}
	if model.Price != nil {
		payload.Price = &publicModelPricePayload{
			Currency:                     model.Price.Currency,
			InputMicroCNYPer1MTokens:     model.Price.InputMicroCNYPer1MTokens,
			OutputMicroCNYPer1MTokens:    model.Price.OutputMicroCNYPer1MTokens,
			CacheReadMicroCNYPer1MTokens: model.Price.CacheReadMicroCNYPer1MTokens,
		}
	}
	if model.Metrics != nil {
		metric := marshalPublicModelMetric(*model.Metrics)
		payload.Metrics = &metric
	}
	return payload
}

func marshalPublicModelMetric(metric repository.PublicModelMetric) publicModelMetricPayload {
	return publicModelMetricPayload{
		Window:        metric.Window,
		Availability:  metric.Availability,
		TTFTP50MS:     metric.TTFTP50MS,
		TTFTP95MS:     metric.TTFTP95MS,
		ResponseSpeed: metric.ResponseSpeed,
		SuccessRate:   metric.SuccessRate,
		SampleCount:   metric.SampleCount,
		UpdatedAt:     metric.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
