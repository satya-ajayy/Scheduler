package response

import (
	"encoding/json"
	"net/http"

	errors "scheduler/internal/errors"
)

// RespondJSON encodes data as JSON and writes it with the given status code.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, `{"message":"internal error encoding response"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// RespondMessage writes a JSON message body with the given status code.
func RespondMessage(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"message": message})
}

// RespondError maps a typed application error to an HTTP response.
func RespondError(w http.ResponseWriter, err *errors.Error) {
	switch err.Kind {
	case errors.NotFound:
		RespondMessage(w, http.StatusNotFound, err.Message)
	case errors.Conflict:
		RespondMessage(w, http.StatusConflict, err.Message)
	case errors.Invalid:
		var ve errors.ValidationErrors
		if errors.As(err, &ve) {
			RespondJSON(w, http.StatusBadRequest, map[string]any{
				"message":           err.Message,
				"validation_errors": ve,
			})
			return
		}
		if err.WrappedErr != nil {
			RespondMessage(w, http.StatusBadRequest, err.WrappedErr.Error())
			return
		}
		RespondMessage(w, http.StatusBadRequest, err.Message)
	case errors.Unauthorized:
		RespondMessage(w, http.StatusUnauthorized, err.Message)
	case errors.Forbidden:
		RespondMessage(w, http.StatusForbidden, err.Message)
	default:
		RespondMessage(w, http.StatusInternalServerError, err.Message)
	}
}
