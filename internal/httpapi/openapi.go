package httpapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPIYAML []byte

func (s *Server) openapiYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPIYAML)
}

func (s *Server) openapiJSON(w http.ResponseWriter, r *http.Request) {
	// Serve YAML under the JSON path as well for clients that only know one URL;
	// content type stays YAML so tooling can detect it. A full YAML→JSON converter
	// is intentionally avoided to keep the binary dependency-free.
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="openapi.yaml"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPIYAML)
}
