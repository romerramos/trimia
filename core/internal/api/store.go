package api

import (
	"sync"
	"time"

	"romerramos/trimia/internal/trimia"
)

type mediaRecord struct {
	ID              string    `json:"mediaId"`
	Filename        string    `json:"filename"`
	ContentType     string    `json:"contentType"`
	SizeBytes       int64     `json:"sizeBytes"`
	DurationSeconds float64   `json:"durationSeconds"`
	Status          string    `json:"status"`
	PreviewStatus   string    `json:"previewStatus"`
	PreviewProgress float64   `json:"previewProgress"`
	PreviewError    string    `json:"previewError,omitempty"`
	Path            string    `json:"-"`
	PreviewPath     string    `json:"-"`
	CreatedAt       time.Time `json:"createdAt"`
}

type jobRecord struct {
	ID        string                `json:"jobId"`
	MediaID   string                `json:"mediaId"`
	Status    string                `json:"status"`
	Phase     string                `json:"phase"`
	Progress  float64               `json:"progress"`
	Options   trimia.AnalyzeOptions `json:"-"`
	Analysis  *analysisResponse     `json:"analysis,omitempty"`
	Segments  []segmentResponse     `json:"-"`
	Version   int                   `json:"-"`
	Error     string                `json:"error,omitempty"`
	Render    *renderRecord         `json:"-"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

type renderRecord struct {
	ID       string                `json:"renderId"`
	JobID    string                `json:"jobId"`
	Status   string                `json:"status"`
	Phase    string                `json:"phase"`
	Progress float64               `json:"progress"`
	Result   *renderResultResponse `json:"result,omitempty"`
	Error    string                `json:"error,omitempty"`
}

type store struct {
	mu      sync.RWMutex
	dataDir string
	media   map[string]*mediaRecord
	jobs    map[string]*jobRecord
}

func newStore(dataDir string) *store {
	return &store{
		dataDir: dataDir,
		media:   make(map[string]*mediaRecord),
		jobs:    make(map[string]*jobRecord),
	}
}

func (s *Server) lookupMedia(id string) (*mediaRecord, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	media, ok := s.store.media[id]
	if !ok {
		return nil, false
	}
	copy := *media
	return &copy, true
}

func (s *Server) updateMediaPreviewProgress(id, status string, progress float64, errorMessage string) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	media := s.store.media[id]
	if media == nil {
		return
	}
	media.PreviewStatus = status
	media.PreviewProgress = progress
	media.PreviewError = errorMessage
	media.Status = status
}

func (s *Server) lookupJob(id string) (*jobRecord, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	job, ok := s.store.jobs[id]
	if !ok {
		return nil, false
	}

	copy := *job
	if job.Render != nil {
		renderCopy := *job.Render
		copy.Render = &renderCopy
	}
	return &copy, true
}

func (s *Server) updateJobProgress(jobID, status, phase string, progress float64, errorMessage string) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	job := s.store.jobs[jobID]
	job.Status = status
	job.Phase = phase
	job.Progress = progress
	job.Error = errorMessage
	job.UpdatedAt = time.Now().UTC()
}

func (s *Server) updateRenderProgress(jobID, status, phase string, progress float64, errorMessage string) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	job := s.store.jobs[jobID]
	if status == "failed" {
		job.Status = "failed"
	} else {
		job.Status = "rendering"
	}
	job.Phase = phase
	job.Progress = progress
	job.Error = errorMessage
	job.UpdatedAt = time.Now().UTC()
	if job.Render != nil {
		job.Render.Status = status
		job.Render.Phase = phase
		job.Render.Progress = progress
		job.Render.Error = errorMessage
	}
}
