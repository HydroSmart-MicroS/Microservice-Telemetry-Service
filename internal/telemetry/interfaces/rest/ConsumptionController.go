package rest

import (
	"context"
	"encoding/json"
	"errors"
	"hydrosmart/telemetry/internal/telemetry/domain/model/queries"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"hydrosmart/telemetry/internal/telemetry/interfaces/rest/transform"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TokenVerifier interface {
	Verify(context.Context, string) error
}
type ConsumptionController struct {
	Queries services.ConsumptionQueryService
	Tokens  TokenVerifier
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (c ConsumptionController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if err := c.Tokens.Verify(r.Context(), token); err != nil {
		status := 401
		if errors.Is(err, valueobjects.ErrUnavailable) {
			status = 503
		}
		jsonResponse(w, status, map[string]string{"error": "authentication_failed"})
		return
	}
	query := queries.GetConsumptionQuery{Token: token, UnitID: r.URL.Query().Get("unitId"), From: time.Now().UTC().Add(-24 * time.Hour), To: time.Now().UTC(), Limit: 100}
	for key, target := range map[string]*time.Time{"from": &query.From, "to": &query.To} {
		if value := r.URL.Query().Get(key); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				jsonResponse(w, 400, map[string]string{"error": "invalid_period"})
				return
			}
			*target = parsed
		}
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid_limit"})
			return
		}
		query.Limit = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := c.Queries.Handle(ctx, query, strings.HasSuffix(r.URL.Path, "/readings"))
	if err != nil {
		status := 503
		switch {
		case errors.Is(err, valueobjects.ErrInvalid):
			status = 400
		case errors.Is(err, valueobjects.ErrNotFound):
			status = 404
		case errors.Is(err, valueobjects.ErrUnauthorized):
			status = 401
		}
		jsonResponse(w, status, map[string]string{"error": http.StatusText(status)})
		return
	}
	jsonResponse(w, 200, transform.ToResource(query.UnitID, result))
}
