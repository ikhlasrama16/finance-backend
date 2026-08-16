package report

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type reporter interface {
	Generate(context.Context, Request) (Response, error)
}

type Handler struct{ service reporter }

func NewHandler(service reporter) *Handler { return &Handler{service: service} }

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	var input Request
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	response, err := h.service.Generate(r.Context(), input)
	if errors.Is(err, ErrInvalidPeriod) || errors.Is(err, ErrCustomDates) || errors.Is(err, ErrInvalidDate) || errors.Is(err, ErrInvalidDateRange) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": response})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
