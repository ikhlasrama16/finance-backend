package report

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type V2Handler struct{ service *V2Service }

func NewV2Handler(service *V2Service) *V2Handler { return &V2Handler{service: service} }

func (h *V2Handler) Statistics(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result, err := h.service.Statistics(r.Context(), StatisticsRequest{
		StartDate: query.Get("start_date"), EndDate: query.Get("end_date"),
		Comparison: ComparisonInput{Mode: query.Get("comparison"), StartDate: query.Get("comparison_start_date"), EndDate: query.Get("comparison_end_date")},
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": result})
}

func (h *V2Handler) CreateAIJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	var input CreateAIJobRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	job, existed, err := h.service.CreateAIJob(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	status := http.StatusAccepted
	if existed && job.Status == "complete" {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"success": true, "data": job})
}

func (h *V2Handler) GetAIJob(w http.ResponseWriter, r *http.Request) {
	job, found, err := h.service.GetAIJob(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "AI report not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": job})
}

func (h *V2Handler) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrSnapshotStale) {
		writeJSON(w, http.StatusConflict, map[string]any{"success": false, "error": err.Error()})
		return
	}
	if errors.Is(err, ErrRangeDates) || errors.Is(err, ErrFutureDate) || errors.Is(err, ErrComparisonMode) || errors.Is(err, ErrComparisonDates) || errors.Is(err, ErrInvalidDate) || errors.Is(err, ErrInvalidDateRange) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": err.Error()})
		return
	}
	log.Printf("[report_v2] internal server error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
}
