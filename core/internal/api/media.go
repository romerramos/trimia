package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"romerramos/trimia/pkg/ffmpeg"
	"time"
)

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	if ok, reason := s.authorizeUpload(r); !ok {
		s.logger.UploadRejected(reason)
		writeError(w, http.StatusUnauthorized, "invalid upload token", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse multipart upload", err)
		return
	}

	file, header, err := nextFilePart(reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read file field", err)
		return
	}
	defer file.Close()

	id := newID("med")
	filename := filepath.Base(header.Filename)
	path := filepath.Join(s.store.dataDir, "uploads", id+filepath.Ext(filename))
	out, err := os.Create(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create upload", err)
		return
	}

	size, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(copyErr, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "upload too large", copyErr)
			return
		}
		writeError(w, http.StatusInternalServerError, "save upload", copyErr)
		return
	}
	if closeErr != nil {
		writeError(w, http.StatusInternalServerError, "close upload", closeErr)
		return
	}

	duration, err := ffmpeg.ProbeDuration(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "probe uploaded media", err)
		return
	}

	previewPath := filepath.Join(s.store.dataDir, "previews", id+".mp4")
	audioPath := filepath.Join(s.store.dataDir, "audio", id+".mp3")
	waveformPath := filepath.Join(s.store.dataDir, "waveforms", id+".json")
	createProxy := shouldCreatePreviewProxy(r)
	status := "ready"
	previewStatus := "skipped"
	previewProgress := 100.0
	if createProxy {
		status = "proxying"
		previewStatus = "proxying"
		previewProgress = 0
	}

	record := &mediaRecord{
		ID:              id,
		Filename:        filename,
		ContentType:     header.Header.Get("Content-Type"),
		SizeBytes:       size,
		DurationSeconds: duration,
		Status:          status,
		PreviewStatus:   previewStatus,
		PreviewProgress: previewProgress,
		AudioStatus:     "extracting",
		WaveformStatus:  "generating",
		Path:            path,
		AudioPath:       audioPath,
		PreviewPath:     previewPath,
		WaveformPath:    waveformPath,
		CreatedAt:       time.Now().UTC(),
	}

	s.store.mu.Lock()
	s.store.media[id] = record
	_ = s.store.saveLocked()
	s.store.mu.Unlock()
	s.logger.UploadSaved(record)

	if createProxy {
		go s.runPreviewProxy(context.Background(), id)
	}
	go s.runMediaAudioPreparation(context.Background(), id)
	writeJSON(w, http.StatusCreated, record)
}

func shouldCreatePreviewProxy(r *http.Request) bool {
	value := r.URL.Query().Get("proxy")
	return value == "1" || value == "true"
}

func (s *Server) handleMediaItem(w http.ResponseWriter, r *http.Request) {
	mediaID, action := splitMediaPath(r.URL.Path)
	if mediaID == "" || action != "" {
		writeError(w, http.StatusNotFound, "endpoint not found", nil)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	media, ok := s.lookupMedia(mediaID)
	if !ok {
		writeError(w, http.StatusNotFound, "media not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, media)
}

func (s *Server) runPreviewProxy(ctx context.Context, mediaID string) {
	media, ok := s.lookupMedia(mediaID)
	if !ok {
		return
	}

	err := ffmpeg.CreatePreviewProxy(ctx, ffmpeg.PreviewProxyOptions{
		InputPath:  media.Path,
		OutputPath: media.PreviewPath,
		Overwrite:  true,
		Duration:   media.DurationSeconds,
		Progress: func(percent float64) {
			s.updateMediaPreviewProgress(mediaID, "proxying", percent, "")
		},
	})
	if err != nil {
		s.updateMediaPreviewProgress(mediaID, "preview_failed", 100, err.Error())
		return
	}

	s.updateMediaPreviewProgress(mediaID, "preview_ready", 100, "")
}

func (s *Server) runMediaAudioPreparation(ctx context.Context, mediaID string) {
	media, ok := s.lookupMedia(mediaID)
	if !ok {
		return
	}

	if err := ffmpeg.ExtractAudio(ctx, ffmpeg.ExtractAudioOptions{
		InputPath:  media.Path,
		OutputPath: media.AudioPath,
		Overwrite:  true,
		Duration:   media.DurationSeconds,
	}); err != nil {
		s.updateMediaAudioStatus(mediaID, "audio_failed", err.Error())
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", "audio extraction failed: "+err.Error())
		return
	}

	s.updateMediaAudioStatus(mediaID, "audio_ready", "")
	s.runWaveformGeneration(ctx, mediaID)
}

type waveformResponse struct {
	MediaID          string      `json:"mediaId"`
	DurationSeconds  float64     `json:"durationSeconds"`
	SamplesPerSecond int         `json:"samplesPerSecond"`
	Peaks            [][]float64 `json:"peaks"`
}

func (s *Server) runWaveformGeneration(ctx context.Context, mediaID string) {
	media, ok := s.lookupMedia(mediaID)
	if !ok {
		return
	}

	if media.AudioStatus != "audio_ready" {
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", "audio is not ready")
		return
	}

	waveform, err := ffmpeg.GenerateWaveform(ctx, ffmpeg.WaveformOptions{InputPath: media.AudioPath})
	if err != nil {
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", err.Error())
		return
	}

	file, err := os.Create(media.WaveformPath)
	if err != nil {
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", err.Error())
		return
	}
	encodeErr := json.NewEncoder(file).Encode(waveformResponse{
		MediaID:          media.ID,
		DurationSeconds:  media.DurationSeconds,
		SamplesPerSecond: waveform.SamplesPerSecond,
		Peaks:            waveform.Peaks,
	})
	closeErr := file.Close()
	if encodeErr != nil {
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", encodeErr.Error())
		return
	}
	if closeErr != nil {
		s.updateMediaWaveformStatus(mediaID, "waveform_failed", closeErr.Error())
		return
	}

	s.updateMediaWaveformStatus(mediaID, "waveform_ready", "")
}

func (s *Server) handleMediaWaveform(w http.ResponseWriter, r *http.Request) {
	mediaID, action := splitMediaPath(r.URL.Path)
	if mediaID == "" || action != "waveform" {
		writeError(w, http.StatusNotFound, "endpoint not found", nil)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	media, ok := s.lookupMedia(mediaID)
	if !ok {
		writeError(w, http.StatusNotFound, "media not found", nil)
		return
	}
	if media.WaveformStatus != "waveform_ready" {
		writeError(w, http.StatusConflict, "waveform is not ready", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, media.WaveformPath)
}

func (s *Server) handleMediaSource(w http.ResponseWriter, r *http.Request) {
	mediaID, action := splitMediaPath(r.URL.Path)
	if mediaID == "" || (action != "source" && action != "preview") {
		writeError(w, http.StatusNotFound, "endpoint not found", nil)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	media, ok := s.lookupMedia(mediaID)
	if !ok {
		writeError(w, http.StatusNotFound, "media not found", nil)
		return
	}

	path := media.Path
	contentType := media.ContentType
	if action == "preview" && media.PreviewPath != "" {
		if _, err := os.Stat(media.PreviewPath); err == nil {
			path = media.PreviewPath
			contentType = "video/mp4"
		}
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeFile(w, r, path)
}

func nextFilePart(reader *multipart.Reader) (*multipart.Part, *multipart.FileHeader, error) {
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, nil, err
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		return part, &multipart.FileHeader{
			Filename: part.FileName(),
			Header:   part.Header,
		}, nil
	}
}
