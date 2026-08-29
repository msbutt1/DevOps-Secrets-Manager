package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// decodeJSON reads a single JSON object from the request body into dst. Bodies larger than
// maxJSONBodyBytes, unknown fields, malformed JSON and trailing data are rejected with a JSON
// error response, and false is returned.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				fmt.Sprintf("Request body must be at most %d bytes", maxJSONBodyBytes))
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.TrimPrefix(err.Error(), "json: unknown field ")
			writeError(w, http.StatusBadRequest, "invalid_request", "Unknown field "+field)
		case errors.As(err, &typeErr):
			writeError(w, http.StatusBadRequest, "invalid_request",
				fmt.Sprintf("Field %q must be %s", typeErr.Field, typeErr.Type))
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be a JSON object")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		}
		return false
	}
	if dec.More() {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must contain a single JSON object")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must contain a single JSON object")
		return false
	}
	return true
}

// rejectInvalid writes a 400 validation_failed response for a validation error and reports
// whether the handler should stop.
func rejectInvalid(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	return true
}
