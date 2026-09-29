package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

const maxUpload = 25 << 20

func (s *Server) listAttachments(w http.ResponseWriter, r *http.Request) {
	entityType := strings.TrimSpace(r.URL.Query().Get("entity_type"))
	id, ok := queryUUID(w, r, "entity_id")
	if !ok {
		return
	}
	if entityType == "" || id == nil {
		writeError(w, http.StatusBadRequest, "invalid", "entity_type and entity_id are required.")
		return
	}
	rows, err := s.store.ListAttachments(r.Context(), entityType, *id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+4096)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "File is larger than 25 MB.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid", "Upload must be multipart form data with a file field.")
		return
	}
	entityType := strings.TrimSpace(r.FormValue("entity_type"))
	entityID, err := uuid.Parse(strings.TrimSpace(r.FormValue("entity_id")))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "entity_id is not a valid id.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "file is required.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "file is empty.")
		return
	}
	if len(data) > maxUpload {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "File is larger than 25 MB.")
		return
	}
	contentType := sniff(data)
	if contentType == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "File type must be JPEG, PNG, WebP, PDF, or plain text.")
		return
	}
	sum := sha256.Sum256(data)
	id := uuid.New()
	dir := filepath.Join(s.cfg.DataDir, "attachments")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.fail(w, r, err)
		return
	}
	storage := id.String()
	finalPath := filepath.Join(dir, storage)
	tmp := finalPath + ".partial"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := os.Rename(tmp, finalPath); err != nil {
		_ = os.Remove(tmp)
		s.fail(w, r, err)
		return
	}
	saved, err := s.store.InsertAttachment(r.Context(), s.actor(r), domain.Attachment{
		ID: id, EntityType: entityType, EntityID: entityID, FileName: safeFilename(header.Filename),
		ContentType: contentType, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
	}, storage)
	if err != nil {
		_ = os.Remove(finalPath)
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/attachments/"+saved.ID.String())
	writeData(w, http.StatusCreated, saved)
}

func (s *Server) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	a, storage, err := s.store.GetAttachment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	storage = filepath.Base(storage)
	f, err := os.Open(filepath.Join(s.cfg.DataDir, "attachments", storage))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "File is no longer on disk.")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename(a.FileName)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, safeFilename(a.FileName), a.CreatedAt, f)
}

func (s *Server) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	storage, err := s.store.DeleteAttachment(r.Context(), s.actor(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if storage != "" {
		_ = os.Remove(filepath.Join(s.cfg.DataDir, "attachments", filepath.Base(storage)))
	}
	w.WriteHeader(http.StatusNoContent)
}

func sniff(b []byte) string {
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "image/webp"
	}
	ct := http.DetectContentType(b)
	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		return "image/jpeg"
	case strings.HasPrefix(ct, "image/png"):
		return "image/png"
	case strings.HasPrefix(ct, "application/pdf"), bytes.HasPrefix(b, []byte("%PDF")):
		return "application/pdf"
	case strings.HasPrefix(ct, "text/plain"):
		sample := b
		if len(sample) > 512 {
			sample = sample[:512]
		}
		low := bytes.ToLower(sample)
		if bytes.Contains(low, []byte("<html")) || bytes.Contains(low, []byte("<script")) || bytes.Contains(low, []byte("<svg")) {
			return ""
		}
		return "text/plain"
	default:
		return ""
	}
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if len(name) > 180 {
		name = name[:180]
	}
	return name
}
