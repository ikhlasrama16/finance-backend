package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Handler struct {
	service *Service
}

func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || accountID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid account id"})
		return
	}
	var request struct {
		ActualBalance *int64 `json:"actual_balance"`
		Note          string `json:"note"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.ActualBalance == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	result, err := h.service.Reconcile(r.Context(), accountID, *request.ActualBalance, request.Note)
	switch {
	case errors.Is(err, ErrInvalidAccountID), errors.Is(err, ErrInvalidReconciliationNote):
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": err.Error()})
	case errors.Is(err, ErrAccountNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "account not found"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": result})
	}
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.service.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    accounts,
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid request body",
		})
		return
	}

	account, err := h.service.Create(r.Context(), input)

	if errors.Is(err, ErrInvalidName) ||
		errors.Is(err, ErrInvalidType) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data":    account,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
