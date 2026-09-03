package notification

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	notifications, err := h.service.List(r.Context(), 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": notifications})
}

func (h *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var input CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]any{
				"success": false,
				"error":   "invalid request body",
			},
		)
		return
	}

	result, err := h.service.Ingest(
		r.Context(),
		input,
	)

	switch {
	case errors.Is(err, ErrDuplicate):
		writeJSON(
			w,
			http.StatusConflict,
			map[string]any{
				"success": false,
				"error":   err.Error(),
			},
		)
		return

	case errors.Is(err, ErrSourceAppRequired),
		errors.Is(err, ErrBodyRequired),
		errors.Is(err, ErrInvalidReceivedAt):

		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]any{
				"success": false,
				"error":   err.Error(),
			},
		)
		return

	case err != nil:
		writeJSON(
			w,
			http.StatusInternalServerError,
			map[string]any{
				"success": false,
				"error":   "internal server error",
			},
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		map[string]any{"success": true, "data": result},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
