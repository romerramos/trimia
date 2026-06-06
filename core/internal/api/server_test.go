package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServerRequiresDeepgramAPIKey(t *testing.T) {
	_, err := NewServer(Options{DataDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHealthEndpoint(t *testing.T) {
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCreateJobRejectsUnknownMedia(t *testing.T) {
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`{"mediaId":"missing","options":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSplitJobPath(t *testing.T) {
	jobID, action := splitJobPath("/api/jobs/job_123/segments")
	if jobID != "job_123" || action != "segments" {
		t.Fatalf("got jobID=%q action=%q", jobID, action)
	}
}
