package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
)

type Summary struct {
	WorkspaceID         string
	RequestCount        int64
	SuccessCount        int64
	ErrorCount          int64
	InputTokens         int64
	OutputTokens        int64
	CachedTokens        int64
	CacheCreationTokens int64
	CostCNY             string
	AvgLatencyMs        int64
	AvgTTFTMs           int64
	Models              []ModelSummary
}

type ModelSummary struct {
	Model        string
	RequestCount int64
	SuccessCount int64
	ErrorCount   int64
	InputTokens  int64
	OutputTokens int64
	CostCNY      string
}

type RequestLog struct {
	RequestID           string
	Time                time.Time
	Model               string
	APIKeyID            string
	APIKeyDisplay       string
	StatusCode          int16
	LatencyMs           int64
	TTFTMs              int64
	InputTokens         int64
	OutputTokens        int64
	CachedTokens        int64
	CacheCreationTokens int64
	CostCNY             string
	ErrorMessage        string
}

type Reader interface {
	Available() bool
	Summary(ctx context.Context, workspaceID string, day time.Time) (Summary, error)
	RecentLogs(ctx context.Context, workspaceID string, limit int) ([]RequestLog, error)
}

type DisabledReader struct{}

func (DisabledReader) Available() bool { return false }

func (DisabledReader) Summary(context.Context, string, time.Time) (Summary, error) {
	return Summary{}, nil
}

func (DisabledReader) RecentLogs(context.Context, string, int) ([]RequestLog, error) {
	return nil, nil
}

type ClickHouseReader struct {
	conn clickhouse.Conn
}

func NewClickHouseReader(cfg config.ClickHouseConfig) (Reader, func() error, error) {
	if !cfg.Enabled || len(cfg.Addr) == 0 {
		return DisabledReader{}, func() error { return nil }, nil
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addr,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open clickhouse: %w", err)
	}
	return &ClickHouseReader{conn: conn}, conn.Close, nil
}

func (r *ClickHouseReader) Available() bool {
	return r != nil && r.conn != nil
}

func (r *ClickHouseReader) Summary(ctx context.Context, workspaceID string, day time.Time) (Summary, error) {
	start, end := dayBounds(day)
	summary := Summary{WorkspaceID: workspaceID}
	err := r.conn.QueryRow(ctx, `
		SELECT
			count(),
			countIf(status_code >= 200 AND status_code < 400),
			countIf(status_code < 200 OR status_code >= 400),
			sum(input_tokens),
			sum(output_tokens),
			sum(cached_tokens),
			sum(cache_creation_tokens),
			toString(sum(cost)),
			toInt64(ifNull(avgOrNull(latency_ms), 0)),
			toInt64(ifNull(avgOrNull(ttft_ms), 0))
		FROM access_logs
		WHERE workspace_id = ? AND time >= ? AND time < ?
	`, workspaceID, start, end).Scan(
		&summary.RequestCount,
		&summary.SuccessCount,
		&summary.ErrorCount,
		&summary.InputTokens,
		&summary.OutputTokens,
		&summary.CachedTokens,
		&summary.CacheCreationTokens,
		&summary.CostCNY,
		&summary.AvgLatencyMs,
		&summary.AvgTTFTMs,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("query usage summary: %w", err)
	}

	rows, err := r.conn.Query(ctx, `
		SELECT
			model,
			count(),
			countIf(status_code >= 200 AND status_code < 400),
			countIf(status_code < 200 OR status_code >= 400),
			sum(input_tokens),
			sum(output_tokens),
			toString(sum(cost))
		FROM access_logs
		WHERE workspace_id = ? AND time >= ? AND time < ?
		GROUP BY model
		ORDER BY count() DESC, model ASC
	`, workspaceID, start, end)
	if err != nil {
		return Summary{}, fmt.Errorf("query usage models: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var model ModelSummary
		if err := rows.Scan(
			&model.Model,
			&model.RequestCount,
			&model.SuccessCount,
			&model.ErrorCount,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CostCNY,
		); err != nil {
			return Summary{}, fmt.Errorf("scan usage model: %w", err)
		}
		summary.Models = append(summary.Models, model)
	}
	if err := rows.Err(); err != nil {
		return Summary{}, fmt.Errorf("iterate usage models: %w", err)
	}
	return summary, nil
}

func (r *ClickHouseReader) RecentLogs(ctx context.Context, workspaceID string, limit int) ([]RequestLog, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			request_id,
			time,
			model,
			api_key_id,
			api_key,
			status_code,
			latency_ms,
			ttft_ms,
			input_tokens,
			output_tokens,
			cached_tokens,
			cache_creation_tokens,
			toString(cost),
			error_message
		FROM access_logs
		WHERE workspace_id = ?
		ORDER BY time DESC
		LIMIT ?
	`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("query request logs: %w", err)
	}
	defer rows.Close()

	logs := make([]RequestLog, 0)
	for rows.Next() {
		var log RequestLog
		if err := rows.Scan(
			&log.RequestID,
			&log.Time,
			&log.Model,
			&log.APIKeyID,
			&log.APIKeyDisplay,
			&log.StatusCode,
			&log.LatencyMs,
			&log.TTFTMs,
			&log.InputTokens,
			&log.OutputTokens,
			&log.CachedTokens,
			&log.CacheCreationTokens,
			&log.CostCNY,
			&log.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("scan request log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request logs: %w", err)
	}
	return logs, nil
}

func dayBounds(day time.Time) (time.Time, time.Time) {
	year, month, date := day.Date()
	start := time.Date(year, month, date, 0, 0, 0, 0, day.Location())
	return start, start.AddDate(0, 0, 1)
}
