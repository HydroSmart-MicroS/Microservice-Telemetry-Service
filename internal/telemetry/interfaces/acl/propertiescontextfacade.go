package acl

import (
	"context"
	"encoding/json"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"net/http"
	"strings"
	"time"
)

type PropertiesContextFacade struct {
	BaseURL, ServiceToken string
	Client                *http.Client
}

func (p PropertiesContextFacade) get(ctx context.Context, path, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(p.BaseURL, "/")+path, nil)
	if err != nil {
		return valueobjects.ErrUnavailable
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("X-Service-Token", p.ServiceToken)
	}
	res, err := p.Client.Do(req)
	if err != nil {
		return valueobjects.ErrUnavailable
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case 200:
		if json.NewDecoder(res.Body).Decode(target) != nil {
			return valueobjects.ErrUnavailable
		}
		return nil
	case 401:
		return valueobjects.ErrUnauthorized
	case 403, 404:
		return valueobjects.ErrNotFound
	default:
		return valueobjects.ErrUnavailable
	}
}
func (p PropertiesContextFacade) Resolve(ctx context.Context, id string) (entities.DeviceSnapshot, error) {
	var dto struct {
		ID       string `json:"id"`
		UnitID   string `json:"unitId"`
		State    string `json:"state"`
		Category string `json:"category"`
	}
	err := p.get(ctx, "/internal/v1/devices/"+id, "", &dto)
	return entities.DeviceSnapshot{ID: dto.ID, UnitID: dto.UnitID, State: dto.State, Category: dto.Category}, err
}
func (p PropertiesContextFacade) Authorize(ctx context.Context, unit, token string) (entities.UnitAccess, error) {
	var dto struct {
		VisibleFrom *time.Time `json:"visibleFrom"`
	}
	err := p.get(ctx, "/internal/v1/units/"+unit+"/access", token, &dto)
	return entities.UnitAccess{VisibleFrom: dto.VisibleFrom}, err
}

// Callers authorize the unit before requesting its trusted device inventory.
func (p PropertiesContextFacade) Devices(ctx context.Context, unit, token string) ([]entities.DeviceSnapshot, error) {
	var snapshot struct {
		Devices []struct {
			ID       string
			State    string
			Category string
		}
	}
	err := p.get(ctx, "/internal/v1/units/"+unit+"/snapshot", "", &snapshot)
	if err != nil {
		return nil, err
	}
	devices := make([]entities.DeviceSnapshot, 0, len(snapshot.Devices))
	for _, device := range snapshot.Devices {
		devices = append(devices, entities.DeviceSnapshot{ID: device.ID, UnitID: unit, State: device.State, Category: device.Category})
	}
	return devices, nil
}
