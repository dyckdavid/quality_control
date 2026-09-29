package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

func writeData(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, map[string]any{"data": v})
}

func writePage(w http.ResponseWriter, data any, limit, offset, total int) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"page": domain.Page{Limit: limit, Offset: offset, Total: total},
	})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body is too large.")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.")
		return false
	}
	return true
}

// decodeOptional accepts a missing or empty body as an empty object.
func decodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body is too large.")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "The id in the path is not valid.")
		return uuid.Nil, false
	}
	return id, true
}

func queryPage(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	return queryPageMax(w, r, 200, 50)
}

func queryPageMax(w http.ResponseWriter, r *http.Request, max, fallback int) (int, int, bool) {
	limit, offset := fallback, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > max {
			writeError(w, http.StatusBadRequest, "invalid", "limit is outside the allowed range.")
			return 0, 0, false
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid", "offset must be zero or greater.")
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}

func queryUUID(w http.ResponseWriter, r *http.Request, key string) (*uuid.UUID, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", key+" is not a valid id.")
		return nil, false
	}
	return &id, true
}

func queryBool(w http.ResponseWriter, r *http.Request, key string) (*bool, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil, true
	}
	switch strings.ToLower(v) {
	case "true", "1":
		t := true
		return &t, true
	case "false", "0":
		f := false
		return &f, true
	default:
		writeError(w, http.StatusBadRequest, "invalid", key+" must be true or false.")
		return nil, false
	}
}

func queryTime(w http.ResponseWriter, r *http.Request, key string) (*time.Time, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", key+" must be an RFC3339 timestamp.")
		return nil, false
	}
	return &t, true
}
