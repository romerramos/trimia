package api

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"romerramos/trimia/pkg/ffmpeg"
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

	record := &mediaRecord{
		ID:              id,
		Filename:        filename,
		ContentType:     header.Header.Get("Content-Type"),
		SizeBytes:       size,
		DurationSeconds: duration,
		Status:          "ready",
		Path:            path,
		CreatedAt:       time.Now().UTC(),
	}

	s.store.mu.Lock()
	s.store.media[id] = record
	s.store.mu.Unlock()
	s.logger.UploadSaved(record)

	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) handleMediaSource(w http.ResponseWriter, r *http.Request) {
	mediaID, action := splitMediaPath(r.URL.Path)
	if mediaID == "" || action != "source" {
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

	if media.ContentType != "" {
		w.Header().Set("Content-Type", media.ContentType)
	}
	http.ServeFile(w, r, media.Path)
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
