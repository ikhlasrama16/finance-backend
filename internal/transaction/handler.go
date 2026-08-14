package transaction

import (
	"encoding/json"
	"errors"
	"net/http"
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
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    transactions,
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

	tx, err := h.service.Create(r.Context(), input)

	switch {
	case errors.Is(err, ErrInvalidType),
		errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrSourceRequired),
		errors.Is(err, ErrDestinationRequired),
		errors.Is(err, ErrSameAccount),
		errors.Is(err, ErrInvalidOccurredAt),
		errors.Is(err, ErrCategoryRequired),
		errors.Is(err, ErrCategoryNotAllowed),
		errors.Is(err, ErrCategoryTypeMismatch):

		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return

	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data":    tx,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
