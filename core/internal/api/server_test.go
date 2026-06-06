package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestCORSPreflight(t *testing.T) {
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: t.TempDir(), AllowedOrigin: "http://localhost:5173"})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/jobs", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") || !strings.Contains(got, "PUT") || !strings.Contains(got, "GET") {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
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

func TestMediaSourceEndpoint(t *testing.T) {
	dataDir := t.TempDir()
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(server.UploadsDir(), "med_test.mp4")
	if err := os.WriteFile(path, []byte("video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	server.store.media["med_test"] = &mediaRecord{
		ID:          "med_test",
		Filename:    "source.mp4",
		ContentType: "video/mp4",
		Path:        path,
		Status:      "ready",
		CreatedAt:   time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test/source", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "video bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
