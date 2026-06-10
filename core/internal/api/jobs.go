package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"romerramos/trimia/internal/trimia"
)

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req createJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "decode request", err)
		return
	}

	media, ok := s.lookupMedia(req.MediaID)
	if !ok {
		writeError(w, http.StatusNotFound, "media not found", nil)
		return
	}

	id := newID("job")
	now := time.Now().UTC()
	job := &jobRecord{
		ID:       id,
		MediaID:  req.MediaID,
		Status:   "queued",
		Phase:    "waiting",
		Progress: 0,
		Options: trimia.AnalyzeOptions{
			InputPath:           media.Path,
			TranscriberProvider: s.transcriberProvider,
			RemoveSilence:       req.Options.RemoveSilence,
			RemoveFillerWords:   req.Options.RemoveFillerWords,
			Language:            req.Options.Language,
			DetectLanguage:      req.Options.DetectLanguage,
			PreRoll:             req.Options.PreRoll,
			PostRoll:            req.Options.PostRoll,
			MergeGap:            req.Options.MergeGap,
		},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.store.mu.Lock()
	s.store.jobs[id] = job
	_ = s.store.saveLocked()
	s.store.mu.Unlock()

	go s.runAnalysis(context.Background(), id)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	jobID, action := splitJobPath(r.URL.Path)
	if jobID == "" {
		writeError(w, http.StatusNotFound, "job not found", nil)
		return
	}

	switch action {
	case "":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleGetJob(w, jobID)
	case "segments":
		s.handleSegments(w, r, jobID)
	case "render":
		s.handleRender(w, r, jobID)
	case "download":
		s.handleDownload(w, r, jobID)
	default:
		writeError(w, http.StatusNotFound, "endpoint not found", nil)
	}
}

func (s *Server) handleGetJob(w http.ResponseWriter, jobID string) {
	job, ok := s.lookupJob(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) runAnalysis(ctx context.Context, jobID string) {
	s.updateJobProgress(jobID, "running", "probing_input", 0, "")
	if err := s.waitForJobAudio(ctx, jobID); err != nil {
		s.updateJobProgress(jobID, "failed", "failed", 100, err.Error())
		return
	}
	result, err := trimia.Analyze(ctx, s.jobAnalyzeOptions(jobID, func(phase string, percent float64) {
		s.updateJobProgress(jobID, "running", apiPhase(phase), percent, "")
	}))
	if err != nil {
		s.updateJobProgress(jobID, "failed", "failed", 100, err.Error())
		return
	}

	segments := make([]segmentResponse, 0, len(result.Segments))
	for _, segment := range result.Segments {
		segments = append(segments, segmentToResponse(segment, string(result.TranscriberProvider)))
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	job := s.store.jobs[jobID]
	job.Status = "awaiting_confirmation"
	job.Phase = "analysis_complete"
	job.Progress = 100
	job.Analysis = &analysisResponse{
		InputDurationSeconds:           result.InputDurationSeconds,
		EstimatedOutputDurationSeconds: result.EstimatedOutputDurationSeconds,
		EstimatedRemovedSeconds:        result.EstimatedRemovedSeconds,
		EstimatedRemovedPercent:        result.EstimatedRemovedPercent,
		OriginalTranscript:             result.OriginalTranscript,
		CleanTranscript:                result.CleanTranscript,
		SegmentsURL:                    "/api/jobs/" + jobID + "/segments",
	}
	job.Segments = segments
	job.Version = 1
	job.UpdatedAt = time.Now().UTC()
	_ = s.store.saveLocked()
}

func (s *Server) jobAnalyzeOptions(jobID string, progress trimia.ProgressFunc) trimia.AnalyzeOptions {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	job := s.store.jobs[jobID]
	opts := job.Options
	opts.Transcriber = s.transcriber
	if media := s.store.media[job.MediaID]; media != nil && media.AudioStatus == "audio_ready" {
		opts.AudioPath = media.AudioPath
	}
	opts.Progress = progress
	return opts
}

func (s *Server) waitForJobAudio(ctx context.Context, jobID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		s.store.mu.RLock()
		job := s.store.jobs[jobID]
		media := s.store.media[job.MediaID]
		status := ""
		errorMessage := ""
		if media != nil {
			status = media.AudioStatus
			errorMessage = media.AudioError
		}
		s.store.mu.RUnlock()

		switch status {
		case "audio_ready":
			return nil
		case "audio_failed":
			if errorMessage == "" {
				errorMessage = "audio extraction failed"
			}
			return errors.New(errorMessage)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
