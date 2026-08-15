package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/core"
)

// TestWriteServiceErrorMapsCodes is a regression test: handlers used to
// collapse service errors into HTTP 400, so forbidden/not-found conditions
// were reported as bad input (the UI then could not distinguish "you may
// not touch this" from "your request is malformed").
func TestWriteServiceErrorMapsCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not_found", core.NotFound("missing"), http.StatusNotFound},
		{"forbidden", core.Forbidden("nope"), http.StatusForbidden},
		{"unauthorized", core.Unauthorized("auth"), http.StatusUnauthorized},
		{"conflict", core.Conflict("conflict"), http.StatusConflict},
		{"bad_request", core.BadRequest("bad"), http.StatusBadRequest},
		{"validation", core.ValidationFailed("bad field"), http.StatusBadRequest},
		{"state_transition", core.StateTransitionInvalid("cannot"), http.StatusBadRequest},
		{"wrapped_api_error", wrapErr(core.Forbidden("wrapped")), http.StatusForbidden},
		{"plain_error", errors.New("boom"), http.StatusInternalServerError},
		{"internal_code", core.Internal("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceError(rec, tc.err)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func wrapErr(err error) error {
	return &wrappedError{err: err}
}

type wrappedError struct{ err error }

func (w *wrappedError) Error() string { return w.err.Error() }
func (w *wrappedError) Unwrap() error { return w.err }
