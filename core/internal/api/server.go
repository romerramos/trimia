package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type Server struct {
	apiKey string
	store  *store
}

type Options struct {
	DeepgramAPIKey string
	DataDir        string
}

func NewServer(opts Options) (*Server, error) {
	if opts.DeepgramAPIKey == "" {
		return nil, errors.New("deepgram api key is required")
	}

	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "trimia-api")
	}

	for _, dir := range []string{dataDir, filepath.Join(dataDir, "uploads"), filepath.Join(dataDir, "outputs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	return &Server{
		apiKey: opts.DeepgramAPIKey,
		store:  newStore(dataDir),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/media", s.handleMedia)
	mux.HandleFunc("/api/jobs", s.handleJobs)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
