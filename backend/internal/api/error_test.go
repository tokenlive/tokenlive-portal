package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, "req_test", ErrWorkspaceNotFound)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	got := body["error"]
	if got["code"] != "workspace.not_found" {
		t.Fatalf("got code %q", got["code"])
	}
	if got["request_id"] != "req_test" {
		t.Fatalf("got request_id %q", got["request_id"])
	}
}
