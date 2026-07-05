package usage

import (
	"context"
	"testing"
	"time"
)

func TestServiceSummaryUnavailableWhenReaderDisabled(t *testing.T) {
	svc := NewService(DisabledReader{}, func() time.Time {
		return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	})

	got, err := svc.Summary(context.Background(), "wsp_1")
	if err != nil {
		t.Fatalf("Summary() err = %v", err)
	}
	if got.Available || got.Today != nil || len(got.Models) != 0 {
		t.Fatalf("Summary() = %+v, want unavailable empty response", got)
	}
	if got.WorkspaceID != "wsp_1" {
		t.Fatalf("WorkspaceID = %q, want wsp_1", got.WorkspaceID)
	}
}

func TestServiceRecentLogsCapsLimitAndHidesHash(t *testing.T) {
	reader := &fakeReader{
		available: true,
		logs: []RequestLog{{
			RequestID:     "req_1",
			Time:          time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
			Model:         "gpt-4o",
			APIKeyID:      "ak_1",
			APIKeyDisplay: "tl_l***1234",
			StatusCode:    200,
			LatencyMs:     123,
		}},
	}
	svc := NewService(reader, nil)

	got, err := svc.RecentLogs(context.Background(), "wsp_1", 500)
	if err != nil {
		t.Fatalf("RecentLogs() err = %v", err)
	}
	if reader.lastLimit != 50 {
		t.Fatalf("limit = %d, want default capped 50", reader.lastLimit)
	}
	if len(got.Logs) != 1 || got.Logs[0].APIKeyID != "ak_1" || got.Logs[0].APIKeyDisplay != "tl_l***1234" {
		t.Fatalf("RecentLogs() = %+v", got)
	}
}

type fakeReader struct {
	available bool
	summary   Summary
	logs      []RequestLog
	lastLimit int
}

func (f *fakeReader) Available() bool { return f.available }

func (f *fakeReader) Summary(context.Context, string, time.Time) (Summary, error) {
	return f.summary, nil
}

func (f *fakeReader) RecentLogs(_ context.Context, _ string, limit int) ([]RequestLog, error) {
	f.lastLimit = limit
	return f.logs, nil
}
