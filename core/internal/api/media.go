package api

import (
	"io"
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

	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("file")
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

	writeJSON(w, http.StatusCreated, record)
}
