package rest

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"net/http/httptest"
	"testing"
)

type failedVerifier struct{ err error }

func (v failedVerifier) Verify(context.Context, string) error { return v.err }
func TestSimulationAuthenticationErrors(t *testing.T) {
	for _, test := range []struct {
		err      error
		expected int
	}{
		{valueobjects.ErrUnavailable, 503}, {valueobjects.ErrUnauthorized, 401},
	} {
		controller := SimulationController{Tokens: failedVerifier{test.err}}
		request := httptest.NewRequest("GET", "/api/v1/simulation/00000000-0000-4000-8000-000000000001", nil)
		request.Header.Set("Authorization", "Bearer test")
		recorder := httptest.NewRecorder()
		controller.ServeHTTP(recorder, request)
		if recorder.Code != test.expected {
			t.Fatalf("got %d, want %d", recorder.Code, test.expected)
		}
	}
}
