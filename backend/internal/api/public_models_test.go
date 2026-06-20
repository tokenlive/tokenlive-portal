package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

func TestPublicModelListHandler(t *testing.T) {
	t.Parallel()

	knowledgeCutoff := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	reader := &fakePublicModelReader{
		listResult: []repository.PublicModel{
			{
				ModelID:          "openai/gpt-5",
				Slug:             "openai-gpt-5",
				Status:           "available",
				DisplayName:      "GPT-5",
				ShortDescription: "Flagship reasoning model.",
				Tags:             []string{"reasoning", "coding"},
				Featured:         true,
				KnowledgeCutoff:  &knowledgeCutoff,
				LogoURL:          "https://example.com/logo.png",
			},
		},
	}

	mux := http.NewServeMux()
	RegisterPublicModelRoutes(mux, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/public/models?locale=en&featured=true&limit=20&offset=5", nil)
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	if reader.listOpts.Locale != "en" {
		t.Fatalf("got locale %q", reader.listOpts.Locale)
	}
	if reader.listOpts.Featured == nil || !*reader.listOpts.Featured {
		t.Fatalf("got featured %#v", reader.listOpts.Featured)
	}
	if reader.listOpts.Limit != 20 {
		t.Fatalf("got limit %d", reader.listOpts.Limit)
	}
	if reader.listOpts.Offset != 5 {
		t.Fatalf("got offset %d", reader.listOpts.Offset)
	}

	var body struct {
		Data []struct {
			ModelID          string   `json:"model_id"`
			Slug             string   `json:"slug"`
			DisplayName      string   `json:"display_name"`
			ShortDescription string   `json:"short_description"`
			Tags             []string `json:"tags"`
			Featured         bool     `json:"featured"`
		} `json:"data"`
		Pagination struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Data) != 1 {
		t.Fatalf("got %d rows, want 1", len(body.Data))
	}
	if body.Data[0].ModelID != "openai/gpt-5" {
		t.Fatalf("got model id %q", body.Data[0].ModelID)
	}
	if body.Pagination.Limit != 20 || body.Pagination.Offset != 5 {
		t.Fatalf("got pagination %+v", body.Pagination)
	}
	if jsonContainsField(t, rec.Body.Bytes(), "knowledge_cutoff") {
		t.Fatalf("list payload unexpectedly contains knowledge_cutoff: %s", rec.Body.String())
	}
	if jsonContainsField(t, rec.Body.Bytes(), "logo_url") {
		t.Fatalf("list payload unexpectedly contains logo_url: %s", rec.Body.String())
	}
}

func TestPublicModelDetailHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	reader := &fakePublicModelReader{
		detailResult: repository.PublicModelDetail{
			PublicModel: repository.PublicModel{
				ModelID:     "openai/gpt-5",
				Slug:        "openai-gpt-5",
				Status:      "available",
				DisplayName: "GPT-5",
				Featured:    true,
			},
			LongDescription: "Long model description.",
			SEOTitle:        "GPT-5 - TokenLive",
			SEODescription:  "SEO description.",
			ServiceMetrics: []repository.PublicModelMetric{
				{
					Window:       "24h",
					SampleCount:  1234,
					UpdatedAt:    now,
					Availability: float64Ptr(0.999),
				},
			},
		},
	}

	mux := http.NewServeMux()
	RegisterPublicModelRoutes(mux, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/public/models/openai-gpt-5?locale=en", nil)
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if reader.detailSlug != "openai-gpt-5" {
		t.Fatalf("got slug %q", reader.detailSlug)
	}
	if reader.detailLocale != "en" {
		t.Fatalf("got locale %q", reader.detailLocale)
	}

	var body struct {
		Data struct {
			ModelID         string `json:"model_id"`
			Slug            string `json:"slug"`
			LongDescription string `json:"long_description"`
			SEOTitle        string `json:"seo_title"`
			ServiceMetrics  []struct {
				Window string `json:"window"`
			} `json:"service_metrics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Data.ModelID != "openai/gpt-5" {
		t.Fatalf("got model id %q", body.Data.ModelID)
	}
	if body.Data.LongDescription != "Long model description." {
		t.Fatalf("got long description %q", body.Data.LongDescription)
	}
	if len(body.Data.ServiceMetrics) != 1 || body.Data.ServiceMetrics[0].Window != "24h" {
		t.Fatalf("got service metrics %#v", body.Data.ServiceMetrics)
	}
}

func TestPublicModelListHandlerReturnsInvalidQuery(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterPublicModelRoutes(mux, &fakePublicModelReader{})

	req := httptest.NewRequest(http.MethodGet, "/api/public/models?limit=oops", nil)
	req.Header.Set("X-Request-ID", "req_invalid_limit")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != "model.invalid_query" {
		t.Fatalf("got code %q", body["error"]["code"])
	}
	if body["error"]["request_id"] != "req_invalid_limit" {
		t.Fatalf("got request_id %q", body["error"]["request_id"])
	}
}

func TestPublicModelDetailHandlerReturnsNotFound(t *testing.T) {
	t.Parallel()

	reader := &fakePublicModelReader{
		detailErr: repository.ErrModelNotFound,
	}

	mux := http.NewServeMux()
	RegisterPublicModelRoutes(mux, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/public/models/missing-model", nil)
	req.Header.Set("X-Request-ID", "req_missing_model")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != "model.not_found" {
		t.Fatalf("got code %q", body["error"]["code"])
	}
	if body["error"]["request_id"] != "req_missing_model" {
		t.Fatalf("got request_id %q", body["error"]["request_id"])
	}
}

type fakePublicModelReader struct {
	listOpts     repository.ModelCatalogListOptions
	listResult   []repository.PublicModel
	listErr      error
	detailSlug   string
	detailLocale string
	detailResult repository.PublicModelDetail
	detailErr    error
}

func (f *fakePublicModelReader) ListPublicModels(_ context.Context, opts repository.ModelCatalogListOptions) ([]repository.PublicModel, error) {
	f.listOpts = opts
	return f.listResult, f.listErr
}

func (f *fakePublicModelReader) GetPublicModelBySlug(_ context.Context, slug string, locale string) (repository.PublicModelDetail, error) {
	f.detailSlug = slug
	f.detailLocale = locale
	if f.detailErr != nil {
		return repository.PublicModelDetail{}, f.detailErr
	}
	return f.detailResult, nil
}

func float64Ptr(v float64) *float64 {
	return &v
}

func jsonContainsField(t *testing.T, body []byte, key string) bool {
	t.Helper()

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode generic payload: %v", err)
	}
	return nestedJSONHasKey(payload, key)
}

func nestedJSONHasKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed[key]; ok {
			return true
		}
		for _, nested := range typed {
			if nestedJSONHasKey(nested, key) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if nestedJSONHasKey(nested, key) {
				return true
			}
		}
	}
	return false
}

func TestFakeReaderImplementsNoUnexpectedBehavior(t *testing.T) {
	t.Parallel()

	reader := &fakePublicModelReader{detailErr: repository.ErrModelNotFound}
	_, err := reader.GetPublicModelBySlug(context.Background(), "slug", "en")
	if !errors.Is(err, repository.ErrModelNotFound) {
		t.Fatalf("got err %v", err)
	}
}
