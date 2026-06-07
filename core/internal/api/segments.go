package api

import (
	"net/http"
	"time"
)

func (s *Server) handleSegments(w http.ResponseWriter, r *http.Request, jobID string) {
	switch r.Method {
	case http.MethodGet:
		job, ok := s.lookupJob(jobID)
		if !ok {
			writeError(w, http.StatusNotFound, "job not found", nil)
			return
		}
		if job.Status != "awaiting_confirmation" && job.Status != "completed" && job.Status != "rendering" {
			writeError(w, http.StatusConflict, "segments are not ready", nil)
			return
		}
		writeJSON(w, http.StatusOK, segmentsResponse{JobID: job.ID, Version: job.Version, Segments: job.Segments, FillerWords: fillerWords(job.Segments)})
	case http.MethodPut:
		var req updateSegmentsRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "decode request", err)
			return
		}
		s.updateSegments(w, jobID, req)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) updateSegments(w http.ResponseWriter, jobID string, req updateSegmentsRequest) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	job, ok := s.store.jobs[jobID]
	if !ok {
		writeError(w, http.StatusNotFound, "job not found", nil)
		return
	}
	if job.Status != "awaiting_confirmation" {
		writeError(w, http.StatusConflict, "job is not awaiting confirmation", nil)
		return
	}
	if req.BaseVersion != job.Version {
		writeError(w, http.StatusConflict, "segment version conflict", nil)
		return
	}
	if len(req.Segments) == 0 {
		writeError(w, http.StatusBadRequest, "segments are required", nil)
		return
	}

	job.Segments = req.Segments
	job.Version++
	job.UpdatedAt = time.Now().UTC()
	_ = s.store.saveLocked()

	inputDuration := job.Analysis.InputDurationSeconds
	estimatedOutput := estimateSegmentDuration(req.Segments, job.Options.PreRoll, job.Options.PostRoll, job.Options.MergeGap)
	estimatedRemoved := inputDuration - estimatedOutput
	estimatedRemovedPercent := 0.0
	if inputDuration > 0 {
		estimatedRemovedPercent = estimatedRemoved / inputDuration * 100
	}

	resp := updateSegmentsResponse{JobID: job.ID, Version: job.Version, Status: job.Status}
	resp.Summary.IncludedSegments = countIncluded(req.Segments)
	resp.Summary.EstimatedOutputDurationSeconds = estimatedOutput
	resp.Summary.EstimatedRemovedSeconds = estimatedRemoved
	resp.Summary.EstimatedRemovedPercent = estimatedRemovedPercent
	writeJSON(w, http.StatusOK, resp)
}
