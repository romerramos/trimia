package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	AudioStatus     string    `json:"audioStatus"`
	AudioError      string    `json:"audioError,omitempty"`
	WaveformStatus  string    `json:"waveformStatus"`
	WaveformError   string    `json:"waveformError,omitempty"`
	Path            string    `json:"-"`
	AudioPath       string    `json:"-"`
	PreviewPath     string    `json:"-"`
	WaveformPath    string    `json:"-"`
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

func newStore(dataDir string) (*store, error) {
	store := &store{
		dataDir: dataDir,
		media:   make(map[string]*mediaRecord),
		jobs:    make(map[string]*jobRecord),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

type storeSnapshot struct {
	Media map[string]*mediaRecord `json:"media"`
	Jobs  map[string]*jobSnapshot `json:"jobs"`
}

type jobSnapshot struct {
	ID        string                 `json:"jobId"`
	MediaID   string                 `json:"mediaId"`
	Status    string                 `json:"status"`
	Phase     string                 `json:"phase"`
	Progress  float64                `json:"progress"`
	Options   analyzeOptionsSnapshot `json:"options"`
	Analysis  *analysisResponse      `json:"analysis,omitempty"`
	Segments  []segmentResponse      `json:"segments,omitempty"`
	Version   int                    `json:"version"`
	Error     string                 `json:"error,omitempty"`
	Render    *renderSnapshot        `json:"render,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

type analyzeOptionsSnapshot struct {
	InputPath         string   `json:"inputPath"`
	AudioPath         string   `json:"audioPath,omitempty"`
	DeepgramAPIKey    string   `json:"-"`
	RemoveSilence     bool     `json:"removeSilence"`
	RemoveFillerWords bool     `json:"removeFillerWords"`
	Language          string   `json:"language"`
	DetectLanguage    bool     `json:"detectLanguage"`
	PreRoll           *float64 `json:"preRoll"`
	PostRoll          *float64 `json:"postRoll"`
	MergeGap          *float64 `json:"mergeGap"`
	KeepTempFiles     bool     `json:"keepTempFiles"`
	LogDir            string   `json:"logDir"`
}

type renderSnapshot struct {
	ID       string                `json:"renderId"`
	JobID    string                `json:"jobId"`
	Status   string                `json:"status"`
	Phase    string                `json:"phase"`
	Progress float64               `json:"progress"`
	Result   *renderResultSnapshot `json:"result,omitempty"`
	Error    string                `json:"error,omitempty"`
}

type renderResultSnapshot struct {
	OutputMediaID         string  `json:"outputMediaId"`
	Filename              string  `json:"filename"`
	DownloadURL           string  `json:"downloadUrl"`
	InputDurationSeconds  float64 `json:"inputDurationSeconds"`
	OutputDurationSeconds float64 `json:"outputDurationSeconds"`
	RemovedSeconds        float64 `json:"removedSeconds"`
	RemovedPercent        float64 `json:"removedPercent"`
	Path                  string  `json:"path"`
}

func (s *store) storePath() string {
	return filepath.Join(s.dataDir, "store.json")
}

func (s *store) load() error {
	contents, err := os.ReadFile(s.storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load store: %w", err)
	}

	var snapshot storeSnapshot
	if err := json.Unmarshal(contents, &snapshot); err != nil {
		return fmt.Errorf("parse store: %w", err)
	}
	if snapshot.Media != nil {
		s.media = snapshot.Media
	}
	if snapshot.Jobs != nil {
		for id, job := range snapshot.Jobs {
			s.jobs[id] = job.restore()
		}
	}
	s.restoreMediaPaths()
	s.markInterruptedWorkFailed()
	return nil
}

func (s *store) saveLocked() error {
	snapshot := storeSnapshot{
		Media: s.media,
		Jobs:  make(map[string]*jobSnapshot, len(s.jobs)),
	}
	for id, job := range s.jobs {
		snapshot.Jobs[id] = snapshotJob(job)
	}

	contents, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	tempPath := s.storePath() + ".tmp"
	if err := os.WriteFile(tempPath, contents, 0o644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	if err := os.Rename(tempPath, s.storePath()); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	return nil
}

func snapshotJob(job *jobRecord) *jobSnapshot {
	return &jobSnapshot{
		ID:       job.ID,
		MediaID:  job.MediaID,
		Status:   job.Status,
		Phase:    job.Phase,
		Progress: job.Progress,
		Options: analyzeOptionsSnapshot{
			InputPath:         job.Options.InputPath,
			AudioPath:         job.Options.AudioPath,
			RemoveSilence:     job.Options.RemoveSilence,
			RemoveFillerWords: job.Options.RemoveFillerWords,
			Language:          job.Options.Language,
			DetectLanguage:    job.Options.DetectLanguage,
			PreRoll:           job.Options.PreRoll,
			PostRoll:          job.Options.PostRoll,
			MergeGap:          job.Options.MergeGap,
			KeepTempFiles:     job.Options.KeepTempFiles,
			LogDir:            job.Options.LogDir,
		},
		Analysis:  job.Analysis,
		Segments:  job.Segments,
		Version:   job.Version,
		Error:     job.Error,
		Render:    snapshotRender(job.Render),
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}

func (j *jobSnapshot) restore() *jobRecord {
	return &jobRecord{
		ID:       j.ID,
		MediaID:  j.MediaID,
		Status:   j.Status,
		Phase:    j.Phase,
		Progress: j.Progress,
		Options: trimia.AnalyzeOptions{
			InputPath:         j.Options.InputPath,
			AudioPath:         j.Options.AudioPath,
			RemoveSilence:     j.Options.RemoveSilence,
			RemoveFillerWords: j.Options.RemoveFillerWords,
			Language:          j.Options.Language,
			DetectLanguage:    j.Options.DetectLanguage,
			PreRoll:           j.Options.PreRoll,
			PostRoll:          j.Options.PostRoll,
			MergeGap:          j.Options.MergeGap,
			KeepTempFiles:     j.Options.KeepTempFiles,
			LogDir:            j.Options.LogDir,
		},
		Analysis:  j.Analysis,
		Segments:  j.Segments,
		Version:   j.Version,
		Error:     j.Error,
		Render:    j.Render.restore(),
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

func snapshotRender(render *renderRecord) *renderSnapshot {
	if render == nil {
		return nil
	}
	return &renderSnapshot{
		ID:       render.ID,
		JobID:    render.JobID,
		Status:   render.Status,
		Phase:    render.Phase,
		Progress: render.Progress,
		Result:   snapshotRenderResult(render.Result),
		Error:    render.Error,
	}
}

func snapshotRenderResult(result *renderResultResponse) *renderResultSnapshot {
	if result == nil {
		return nil
	}
	return &renderResultSnapshot{
		OutputMediaID:         result.OutputMediaID,
		Filename:              result.Filename,
		DownloadURL:           result.DownloadURL,
		InputDurationSeconds:  result.InputDurationSeconds,
		OutputDurationSeconds: result.OutputDurationSeconds,
		RemovedSeconds:        result.RemovedSeconds,
		RemovedPercent:        result.RemovedPercent,
		Path:                  result.path,
	}
}

func (r *renderSnapshot) restore() *renderRecord {
	if r == nil {
		return nil
	}
	return &renderRecord{
		ID:       r.ID,
		JobID:    r.JobID,
		Status:   r.Status,
		Phase:    r.Phase,
		Progress: r.Progress,
		Result:   r.Result.restore(),
		Error:    r.Error,
	}
}

func (r *renderResultSnapshot) restore() *renderResultResponse {
	if r == nil {
		return nil
	}
	return &renderResultResponse{
		OutputMediaID:         r.OutputMediaID,
		Filename:              r.Filename,
		DownloadURL:           r.DownloadURL,
		InputDurationSeconds:  r.InputDurationSeconds,
		OutputDurationSeconds: r.OutputDurationSeconds,
		RemovedSeconds:        r.RemovedSeconds,
		RemovedPercent:        r.RemovedPercent,
		path:                  r.Path,
	}
}

func (s *store) restoreMediaPaths() {
	for _, media := range s.media {
		if media.Path == "" {
			media.Path = filepath.Join(s.dataDir, "uploads", media.ID+filepath.Ext(media.Filename))
		}
		if media.AudioPath == "" {
			media.AudioPath = filepath.Join(s.dataDir, "audio", media.ID+".mp3")
		}
		if media.PreviewPath == "" {
			media.PreviewPath = filepath.Join(s.dataDir, "previews", media.ID+".mp4")
		}
		if media.WaveformPath == "" {
			media.WaveformPath = filepath.Join(s.dataDir, "waveforms", media.ID+".json")
		}
	}
}

func (s *store) markInterruptedWorkFailed() {
	for _, media := range s.media {
		if media.AudioStatus == "extracting" {
			media.AudioStatus = "audio_failed"
			media.AudioError = "audio extraction interrupted by server restart"
		}
		if media.WaveformStatus == "generating" {
			media.WaveformStatus = "waveform_failed"
			media.WaveformError = "waveform generation interrupted by server restart"
		}
		if media.PreviewStatus == "proxying" {
			media.PreviewStatus = "preview_failed"
			media.PreviewError = "preview generation interrupted by server restart"
			media.Status = "preview_failed"
		}
	}
	for _, job := range s.jobs {
		if job.Status == "queued" || job.Status == "running" {
			job.Status = "failed"
			job.Phase = "failed"
			job.Progress = 100
			job.Error = "analysis interrupted by server restart"
		}
		if job.Render != nil && (job.Render.Status == "queued" || job.Render.Status == "running") {
			job.Status = "failed"
			job.Phase = "failed"
			job.Progress = 100
			job.Error = "render interrupted by server restart"
			job.Render.Status = "failed"
			job.Render.Phase = "failed"
			job.Render.Progress = 100
			job.Render.Error = "render interrupted by server restart"
		}
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
	_ = s.store.saveLocked()
}

func (s *Server) updateMediaWaveformStatus(id, status string, errorMessage string) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	media := s.store.media[id]
	if media == nil {
		return
	}
	media.WaveformStatus = status
	media.WaveformError = errorMessage
	_ = s.store.saveLocked()
}

func (s *Server) updateMediaAudioStatus(id, status string, errorMessage string) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	media := s.store.media[id]
	if media == nil {
		return
	}
	media.AudioStatus = status
	media.AudioError = errorMessage
	_ = s.store.saveLocked()
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
	_ = s.store.saveLocked()
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
	_ = s.store.saveLocked()
}
