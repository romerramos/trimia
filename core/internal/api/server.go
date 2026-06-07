package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Server struct {
	apiKey            string
	store             *store
	uploadTokenSecret string
	allowedOrigin     string
	maxUploadBytes    int64
	logger            *logger
}

type Options struct {
	DeepgramAPIKey    string
	DataDir           string
	UploadTokenSecret string
	AllowedOrigin     string
	MaxUploadBytes    int64
	LogFormat         LogFormat
}

const defaultMaxUploadBytes int64 = 5 * 1024 * 1024 * 1024

func NewServer(opts Options) (*Server, error) {
	if opts.DeepgramAPIKey == "" {
		return nil, errors.New("deepgram api key is required")
	}

	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "trimia-api")
	}

	for _, dir := range []string{dataDir, filepath.Join(dataDir, "uploads"), filepath.Join(dataDir, "outputs"), filepath.Join(dataDir, "previews"), filepath.Join(dataDir, "waveforms"), filepath.Join(dataDir, "audio")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	maxUploadBytes := opts.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}

	return &Server{
		apiKey:            opts.DeepgramAPIKey,
		store:             newStore(dataDir),
		uploadTokenSecret: opts.UploadTokenSecret,
		allowedOrigin:     opts.AllowedOrigin,
		maxUploadBytes:    maxUploadBytes,
		logger:            newLogger(opts.LogFormat, os.Stdout),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/media", s.handleMedia)
	mux.HandleFunc("/api/media/", s.handleMediaRoute)
	mux.HandleFunc("/api/jobs", s.handleJobs)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	return s.logRequests(s.cors(mux))
}

func (s *Server) handleMediaRoute(w http.ResponseWriter, r *http.Request) {
	_, action := splitMediaPath(r.URL.Path)
	if action == "source" || action == "preview" {
		s.handleMediaSource(w, r)
		return
	}
	if action == "waveform" {
		s.handleMediaWaveform(w, r)
		return
	}

	s.handleMediaItem(w, r)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := s.allowedOrigin
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)
		s.logger.Request(r.Method, r.URL.Path, wrapped.status, time.Since(started))
	})
}

func (s *Server) DataDir() string {
	return s.store.dataDir
}

func (s *Server) UploadsDir() string {
	return filepath.Join(s.store.dataDir, "uploads")
}

func (s *Server) PreviewsDir() string {
	return filepath.Join(s.store.dataDir, "previews")
}

func (s *Server) WaveformsDir() string {
	return filepath.Join(s.store.dataDir, "waveforms")
}

func (s *Server) AudioDir() string {
	return filepath.Join(s.store.dataDir, "audio")
}

func (s *Server) MaxUploadBytes() int64 {
	return s.maxUploadBytes
}

func (s *Server) UploadAuthEnabled() bool {
	return s.uploadTokenSecret != ""
}

func (s *Server) AllowedOrigin() string {
	if s.allowedOrigin == "" {
		return "*"
	}
	return s.allowedOrigin
}
