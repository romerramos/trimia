package api

import (
	"context"
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
		Path:            path,
		PreviewPath:     previewPath,
		CreatedAt:       time.Now().UTC(),
	}

	s.store.mu.Lock()
	s.store.media[id] = record
	s.store.mu.Unlock()
	s.logger.UploadSaved(record)

	if createProxy {
		go s.runPreviewProxy(context.Background(), id)
	}
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
