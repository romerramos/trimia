package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"romerramos/trimia/internal/trimia"
)

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request, jobID string) {
	switch r.Method {
	case http.MethodPost:
		var req renderRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "decode request", err)
			return
		}
		render, err := s.createRender(jobID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		go s.runRender(context.Background(), jobID, req)
		writeJSON(w, http.StatusAccepted, render)
	case http.MethodGet:
		job, ok := s.lookupJob(jobID)
		if !ok {
			writeError(w, http.StatusNotFound, "job not found", nil)
			return
		}
		if job.Render == nil {
			writeError(w, http.StatusNotFound, "render not found", nil)
			return
		}
		writeJSON(w, http.StatusOK, job.Render)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	job, ok := s.lookupJob(jobID)
	if !ok || job.Render == nil || job.Render.Result == nil {
		writeError(w, http.StatusNotFound, "render output not found", nil)
		return
	}
	http.ServeFile(w, r, job.Render.Result.path)
}

func (s *Server) createRender(jobID string, req renderRequest) (*renderRecord, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	job, ok := s.store.jobs[jobID]
	if !ok {
		return nil, errors.New("job not found")
	}
	if job.Status != "awaiting_confirmation" {
		return nil, errors.New("job is not awaiting confirmation")
	}
	if req.SegmentVersion != job.Version {
		return nil, errors.New("segment version conflict")
	}
	if countIncluded(job.Segments) == 0 {
		return nil, errors.New("at least one included segment is required")
	}

	render := &renderRecord{ID: newID("rnd"), JobID: jobID, Status: "queued", Phase: "waiting", Progress: 0}
	job.Render = render
	job.Status = "rendering"
	job.Phase = "rendering_video"
	job.UpdatedAt = time.Now().UTC()
	_ = s.store.saveLocked()
	return render, nil
}

func (s *Server) runRender(ctx context.Context, jobID string, req renderRequest) {
	job, ok := s.lookupJob(jobID)
	if !ok {
		return
	}
	media, ok := s.lookupMedia(job.MediaID)
	if !ok {
		s.updateRenderProgress(jobID, "failed", "failed", 100, "media not found")
		return
	}

	filename := req.Output.Filename
	if filename == "" {
		filename = strings.TrimSuffix(media.Filename, filepath.Ext(media.Filename)) + "_trimia.mp4"
	}
	outputPath := filepath.Join(s.store.dataDir, "outputs", jobID+"-"+filepath.Base(filename))

	renderSegments := make([]trimia.Segment, 0, len(job.Segments))
	for _, segment := range job.Segments {
		renderSegments = append(renderSegments, responseToSegment(segment))
	}

	result, err := trimia.Render(ctx, trimia.RenderOptions{
		InputPath:  media.Path,
		OutputPath: outputPath,
		Segments:   renderSegments,
		Overwrite:  req.Output.Overwrite,
		Progress: func(phase string, percent float64) {
			s.updateRenderProgress(jobID, "running", apiPhase(phase), percent, "")
		},
		RenderMode:  req.RenderOptions.RenderMode,
		VideoPreset: req.RenderOptions.Preset,
		VideoCRF:    req.RenderOptions.CRF,
		AudioRate:   req.RenderOptions.AudioRate,
	})
	if err != nil {
		s.updateRenderProgress(jobID, "failed", "failed", 100, err.Error())
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	job = s.store.jobs[jobID]
	job.Status = "completed"
	job.Phase = "complete"
	job.Progress = 100
	job.UpdatedAt = time.Now().UTC()
	job.Render.Status = "completed"
	job.Render.Phase = "complete"
	job.Render.Progress = 100
	job.Render.Result = &renderResultResponse{
		OutputMediaID:         newID("out"),
		Filename:              filename,
		DownloadURL:           "/api/jobs/" + jobID + "/download",
		InputDurationSeconds:  result.InputDurationSeconds,
		OutputDurationSeconds: result.OutputDurationSeconds,
		RemovedSeconds:        result.RemovedSeconds,
		RemovedPercent:        result.RemovedPercent,
		path:                  outputPath,
	}
	_ = s.store.saveLocked()
}
