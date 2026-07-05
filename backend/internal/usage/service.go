package usage

import (
	"context"
	"time"
)

type SummaryResponse struct {
	DataSource  string          `json:"data_source"`
	Available   bool            `json:"available"`
	WorkspaceID string          `json:"workspace_id"`
	Today       *TodaySummary   `json:"today"`
	Models      []ModelResponse `json:"models"`
}

type TodaySummary struct {
	RequestCount        int64  `json:"request_count"`
	SuccessCount        int64  `json:"success_count"`
	ErrorCount          int64  `json:"error_count"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CachedTokens        int64  `json:"cached_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CostCNY             string `json:"cost_cny"`
	AvgLatencyMs        int64  `json:"avg_latency_ms"`
	AvgTTFTMs           int64  `json:"avg_ttft_ms"`
}

type ModelResponse struct {
	Model        string `json:"model"`
	RequestCount int64  `json:"request_count"`
	SuccessCount int64  `json:"success_count"`
	ErrorCount   int64  `json:"error_count"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostCNY      string `json:"cost_cny"`
}

type RequestLogsResponse struct {
	Logs []RequestLogResponse `json:"logs"`
}

type RequestLogResponse struct {
	RequestID           string    `json:"request_id"`
	Time                time.Time `json:"time"`
	Model               string    `json:"model"`
	APIKeyID            string    `json:"api_key_id"`
	APIKeyDisplay       string    `json:"api_key_display"`
	StatusCode          int16     `json:"status_code"`
	LatencyMs           int64     `json:"latency_ms"`
	TTFTMs              int64     `json:"ttft_ms"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CachedTokens        int64     `json:"cached_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CostCNY             string    `json:"cost_cny"`
	ErrorMessage        string    `json:"error_message"`
}

type Service struct {
	reader Reader
	now    func() time.Time
}

func NewService(reader Reader, now func() time.Time) *Service {
	if reader == nil {
		reader = DisabledReader{}
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{reader: reader, now: now}
}

func (s *Service) Summary(ctx context.Context, workspaceID string) (SummaryResponse, error) {
	resp := SummaryResponse{
		DataSource:  "clickhouse",
		Available:   s.reader.Available(),
		WorkspaceID: workspaceID,
		Models:      []ModelResponse{},
	}
	if !s.reader.Available() {
		return resp, nil
	}

	summary, err := s.reader.Summary(ctx, workspaceID, s.now())
	if err != nil {
		resp.Available = false
		return resp, nil
	}
	resp.Today = &TodaySummary{
		RequestCount:        summary.RequestCount,
		SuccessCount:        summary.SuccessCount,
		ErrorCount:          summary.ErrorCount,
		InputTokens:         summary.InputTokens,
		OutputTokens:        summary.OutputTokens,
		CachedTokens:        summary.CachedTokens,
		CacheCreationTokens: summary.CacheCreationTokens,
		CostCNY:             summary.CostCNY,
		AvgLatencyMs:        summary.AvgLatencyMs,
		AvgTTFTMs:           summary.AvgTTFTMs,
	}
	for _, model := range summary.Models {
		resp.Models = append(resp.Models, ModelResponse{
			Model:        model.Model,
			RequestCount: model.RequestCount,
			SuccessCount: model.SuccessCount,
			ErrorCount:   model.ErrorCount,
			InputTokens:  model.InputTokens,
			OutputTokens: model.OutputTokens,
			CostCNY:      model.CostCNY,
		})
	}
	return resp, nil
}

func (s *Service) RecentLogs(ctx context.Context, workspaceID string, limit int) (RequestLogsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	resp := RequestLogsResponse{Logs: []RequestLogResponse{}}
	if !s.reader.Available() {
		return resp, nil
	}

	logs, err := s.reader.RecentLogs(ctx, workspaceID, limit)
	if err != nil {
		return resp, nil
	}
	for _, log := range logs {
		resp.Logs = append(resp.Logs, RequestLogResponse{
			RequestID:           log.RequestID,
			Time:                log.Time,
			Model:               log.Model,
			APIKeyID:            log.APIKeyID,
			APIKeyDisplay:       log.APIKeyDisplay,
			StatusCode:          log.StatusCode,
			LatencyMs:           log.LatencyMs,
			TTFTMs:              log.TTFTMs,
			InputTokens:         log.InputTokens,
			OutputTokens:        log.OutputTokens,
			CachedTokens:        log.CachedTokens,
			CacheCreationTokens: log.CacheCreationTokens,
			CostCNY:             log.CostCNY,
			ErrorMessage:        log.ErrorMessage,
		})
	}
	return resp, nil
}
