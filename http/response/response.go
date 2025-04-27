package response

import (
	// Go Internal Packages
	"encoding/json"
	"net/http"

	// Local Packages
	errors "scheduler/errors"
)

// RespondJSON writes the data to the response writer as JSON
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if json.NewEncoder(w).Encode(data) != nil {
		http.Error(w, `{"message": "Internal Error Encoding Response"}`, http.StatusInternalServerError)
	}
}

// RespondMessage writes the message to the response writer
func RespondMessage(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"message": message})
}

// RespondError writes the error to the response writer
func RespondError(w http.ResponseWriter, err *errors.Error) {
	switch err.Kind {
	case errors.NotFound:
		RespondMessage(w, http.StatusNotFound, err.Message)
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
