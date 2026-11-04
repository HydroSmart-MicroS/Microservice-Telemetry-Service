package rest

import (
	"encoding/json"
	"errors"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"io"
	"net/http"
	"strings"
)

type SimulationController struct {
	Service services.SimulationService
	Tokens  TokenVerifier
}

func (c SimulationController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if err := c.Tokens.Verify(r.Context(), token); err != nil {
		status := 401
		if errors.Is(err, valueobjects.ErrUnavailable) {
			status = 503
		}
		jsonResponse(w, status, map[string]string{"error": "authentication_failed"})
		return
	}
	var result entities.SimulationState
	var err error
	unit := r.PathValue("unitID")
	if r.Method == "GET" {
		result, err = c.Service.Status(r.Context(), unit, token)
	} else {
		var body struct {
			Scenario string `json:"scenario"`
			Action   string `json:"action"`
			Days     int    `json:"days"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			jsonResponse(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if body.Action == "scenario" {
			result, err = c.Service.Scenario(r.Context(), unit, token, body.Scenario)
		} else {
			result, err = c.Service.Handle(r.Context(), unit, token, body.Action, body.Days)
		}
	}
	if err != nil {
		status := 503
		switch {
		case errors.Is(err, valueobjects.ErrInvalid):
			status = 400
		case errors.Is(err, valueobjects.ErrForbidden):
			status = 403
		case errors.Is(err, valueobjects.ErrNotFound):
			status = 404
		case errors.Is(err, valueobjects.ErrUnauthorized):
			status = 401
		case errors.Is(err, valueobjects.ErrConflict):
			status = 409
		}
		jsonResponse(w, status, map[string]string{"error": http.StatusText(status)})
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 202
	}
	jsonResponse(w, status, map[string]any{"running": result.Running, "seeding": result.Seeding, "completed": result.Completed, "total": result.Total, "error": result.Error})
}
