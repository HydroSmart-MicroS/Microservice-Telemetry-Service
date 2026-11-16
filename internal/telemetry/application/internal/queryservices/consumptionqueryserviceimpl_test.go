package queryservices

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/queries"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"reflect"
	"testing"
	"time"
)

type queryRegistry struct {
	services.DeviceRegistry
	devices         []entities.DeviceSnapshot
	authError       error
	inventoryCalled bool
}

func (r *queryRegistry) Authorize(context.Context, string, string) (entities.UnitAccess, error) {
	return entities.UnitAccess{}, r.authError
}
func (r *queryRegistry) Devices(context.Context, string, string) ([]entities.DeviceSnapshot, error) {
	r.inventoryCalled = true
	return r.devices, nil
}

type queryReadings struct {
	services.ReadingRepository
	generalIDs []string
}

func (r *queryReadings) Summary(_ context.Context, _ string, _ time.Time, _ time.Time, ids []string) (aggregates.Consumption, error) {
	r.generalIDs = ids
	return aggregates.Consumption{}, nil
}
func TestConsumptionTotalsIncludeRetiredGeneralMetersAndExcludeZones(t *testing.T) {
	unit := "00000000-0000-4000-8000-000000000001"
	registry := &queryRegistry{devices: []entities.DeviceSnapshot{
		{ID: "old", UnitID: unit, Category: "GENERAL", State: "disabled"},
		{ID: "new", UnitID: unit, Category: "GENERAL", State: "active"},
		{ID: "bath", UnitID: unit, Category: "BAÑO", State: "active"},
	}}
	readings := &queryReadings{}
	service := ConsumptionQueryServiceImpl{Registry: registry, Repository: readings}
	q := queries.GetConsumptionQuery{UnitID: unit, From: time.Now().Add(-time.Hour), To: time.Now(), Limit: 100}
	if _, err := service.Handle(context.Background(), q, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(readings.generalIDs, []string{"old", "new"}) {
		t.Fatal("General history must remain included after replacement")
	}
	registry.devices = registry.devices[2:]
	if _, err := service.Handle(context.Background(), q, false); err != nil || len(readings.generalIDs) != 0 {
		t.Fatal("legacy units must retain their historical totals")
	}
	registry.authError = valueobjects.ErrNotFound
	registry.inventoryCalled = false
	if _, err := service.Handle(context.Background(), q, false); err != valueobjects.ErrNotFound || registry.inventoryCalled {
		t.Fatal("authorization must precede inventory lookup")
	}
}
