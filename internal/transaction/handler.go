package transaction

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.service.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": transactions})
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionIDFromPath(w, r)
	if !ok {
		return
	}
	transaction, err := h.service.GetByID(r.Context(), id)
	if errors.Is(err, ErrTransactionNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "transaction not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": transaction})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	transaction, err := h.service.Create(r.Context(), input)
	if err != nil {
		handleCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"success": true, "data": transaction})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionIDFromPath(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	transaction, err := h.service.Update(r.Context(), id, input)
	if err != nil {
		handleMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": transaction})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionIDFromPath(w, r)
	if !ok {
		return
	}
	result, err := h.service.Delete(r.Context(), id)
	if err != nil {
		handleMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": result})
}

func transactionIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid transaction id"})
		return 0, false
	}
	return id, true
}

func handleCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCategoryNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": err.Error()})
	case errors.Is(err, ErrInvalidType), errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrSourceRequired),
		errors.Is(err, ErrDestinationRequired), errors.Is(err, ErrSameAccount), errors.Is(err, ErrInvalidOccurredAt),
		errors.Is(err, ErrCategoryRequired), errors.Is(err, ErrCategoryNotAllowed), errors.Is(err, ErrCategoryTypeMismatch):
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
	}
}

func handleMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTransactionNotFound), errors.Is(err, ErrCategoryNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": err.Error()})
	case errors.Is(err, ErrReconciliationDelete):
		writeJSON(w, http.StatusConflict, map[string]any{"success": false, "error": err.Error()})
	case errors.Is(err, ErrNoUpdateFields), errors.Is(err, ErrCategoryNotAllowed), errors.Is(err, ErrCategoryTypeMismatch):
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
