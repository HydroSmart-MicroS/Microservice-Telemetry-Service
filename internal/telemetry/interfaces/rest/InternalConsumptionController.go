package rest

import (
	"context"
	"crypto/subtle"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"net/http"
)

type ServiceTokenVerifier struct{ Expected string }

func (v ServiceTokenVerifier) Verify(ctx context.Context, token string) error {
	if len(v.Expected) < 32 || subtle.ConstantTimeCompare([]byte(v.Expected), []byte(token)) != 1 {
		return valueobjects.ErrUnauthorized
	}
	return nil
}

type InternalConsumptionController struct {
	Queries services.ConsumptionQueryService
	Secret  string
}

func (c InternalConsumptionController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	verifier := ServiceTokenVerifier{c.Secret}
	if verifier.Verify(r.Context(), r.Header.Get("X-Service-Token")) != nil {
		jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+c.Secret)
	ConsumptionController{Queries: c.Queries, Tokens: verifier}.ServeHTTP(w, clone)
}
