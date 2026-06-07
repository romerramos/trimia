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

func TestMediaItemEndpointReturnsPreviewStatus(t *testing.T) {
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
		ID:              "med_test",
		Filename:        "source.mp4",
		ContentType:     "video/mp4",
		Path:            path,
		Status:          "proxying",
		PreviewStatus:   "proxying",
		PreviewProgress: 42.5,
		CreatedAt:       time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"previewStatus":"proxying"`) || !strings.Contains(body, `"previewProgress":42.5`) {
		t.Fatalf("body = %s", body)
	}
}

func TestShouldCreatePreviewProxy(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/media", want: false},
		{path: "/api/media?proxy=0", want: false},
		{path: "/api/media?proxy=false", want: false},
		{path: "/api/media?proxy=1", want: true},
		{path: "/api/media?proxy=true", want: true},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodPost, test.path, nil)
		if got := shouldCreatePreviewProxy(req); got != test.want {
			t.Fatalf("shouldCreatePreviewProxy(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestMediaPreviewEndpointServesProxy(t *testing.T) {
	dataDir := t.TempDir()
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(server.UploadsDir(), "med_test.mp4")
	if err := os.WriteFile(sourcePath, []byte("source bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	previewPath := filepath.Join(server.PreviewsDir(), "med_test.mp4")
	if err := os.WriteFile(previewPath, []byte("preview bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	server.store.media["med_test"] = &mediaRecord{
		ID:          "med_test",
		Filename:    "source.mp4",
		ContentType: "video/mp4",
		Path:        sourcePath,
		PreviewPath: previewPath,
		Status:      "preview_ready",
		CreatedAt:   time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test/preview", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "video/mp4") {
		t.Fatalf("Content-Type = %q", got)
	}
	if rec.Body.String() != "preview bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestMediaPreviewEndpointFallsBackToSource(t *testing.T) {
	dataDir := t.TempDir()
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(server.UploadsDir(), "med_test.mp4")
	if err := os.WriteFile(path, []byte("source bytes"), 0o644); err != nil {
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

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test/preview", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "source bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestMediaWaveformEndpointServesReadyWaveform(t *testing.T) {
	dataDir := t.TempDir()
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	waveformPath := filepath.Join(server.WaveformsDir(), "med_test.json")
	if err := os.WriteFile(waveformPath, []byte(`{"mediaId":"med_test","peaks":[[-1,1]]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server.store.media["med_test"] = &mediaRecord{
		ID:             "med_test",
		Filename:       "source.mp4",
		ContentType:    "video/mp4",
		WaveformPath:   waveformPath,
		WaveformStatus: "waveform_ready",
		CreatedAt:      time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test/waveform", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"peaks":[[-1,1]]`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestMediaWaveformEndpointReturnsConflictWhenNotReady(t *testing.T) {
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	server.store.media["med_test"] = &mediaRecord{
		ID:             "med_test",
		WaveformStatus: "generating",
		CreatedAt:      time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/med_test/waveform", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestStorePersistsMediaAndJobsAcrossServerRestart(t *testing.T) {
	dataDir := t.TempDir()
	server, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}

	mediaPath := filepath.Join(server.UploadsDir(), "med_test.mp4")
	if err := os.WriteFile(mediaPath, []byte("video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	server.store.mu.Lock()
	server.store.media["med_test"] = &mediaRecord{
		ID:              "med_test",
		Filename:        "source.mp4",
		ContentType:     "video/mp4",
		DurationSeconds: 12,
		Status:          "ready",
		PreviewStatus:   "skipped",
		PreviewProgress: 100,
		AudioStatus:     "audio_ready",
		WaveformStatus:  "waveform_ready",
		Path:            mediaPath,
		AudioPath:       filepath.Join(server.AudioDir(), "med_test.mp3"),
		PreviewPath:     filepath.Join(server.PreviewsDir(), "med_test.mp4"),
		WaveformPath:    filepath.Join(server.WaveformsDir(), "med_test.json"),
		CreatedAt:       time.Now().UTC(),
	}
	server.store.jobs["job_test"] = &jobRecord{
		ID:        "job_test",
		MediaID:   "med_test",
		Status:    "awaiting_confirmation",
		Phase:     "analysis_complete",
		Progress:  100,
		Version:   1,
		Segments:  []segmentResponse{{ID: "seg_test", Start: 0, End: 1, Included: true}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := server.store.saveLocked(); err != nil {
		server.store.mu.Unlock()
		t.Fatal(err)
	}
	server.store.mu.Unlock()

	restarted, err := NewServer(Options{DeepgramAPIKey: "test-key", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.lookupMedia("med_test"); !ok {
		t.Fatal("media was not restored")
	}
	job, ok := restarted.lookupJob("job_test")
	if !ok {
		t.Fatal("job was not restored")
	}
	if job.Version != 1 || len(job.Segments) != 1 {
		t.Fatalf("restored job = %#v", job)
	}
}
