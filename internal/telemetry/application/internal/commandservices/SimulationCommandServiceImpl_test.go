package commandservices

import (
	"context"
	"fmt"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"testing"
	"time"
)

func TestDefaultHistoryCoversPreviousCalendarMonthInLima(t *testing.T) {
	for _, tc := range []struct {
		now, first string
		days       int
	}{
		{"2026-10-31T18:00:00Z", "2026-09-01", 60},
		{"2026-11-01T02:00:00Z", "2026-09-01", 60},
		{"2026-11-01T05:00:00Z", "2026-10-01", 31},
		{"2027-01-31T18:00:00Z", "2026-12-01", 61},
		{"2027-03-01T18:00:00Z", "2027-02-01", 28},
		{"2028-03-01T18:00:00Z", "2028-02-01", 29},
	} {
		t.Run(tc.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			midnight, days := historyWindow(now, 0)
			if days != tc.days || midnight.AddDate(0, 0, -days).Format("2006-01-02") != tc.first {
				t.Fatalf("wrong calendar range: %v, %d days", midnight, days)
			}
			if _, explicit := historyWindow(now, 28); explicit != 28 {
				t.Fatal("explicit lookback must remain compatible")
			}
		})
	}
}

type historyCommands struct{ readings []aggregates.Reading }

func (r *historyCommands) Handle(_ context.Context, command commands.RecordReadingCommand) (bool, error) {
	r.readings = append(r.readings, command.Reading)
	return true, nil
}
func TestCalendarHistoryEmitsFourSamplesPerDayAndStableIDs(t *testing.T) {
	now := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	midnight, days := historyWindow(now, 0)
	commands := &historyCommands{}
	service := &SimulationCommandServiceImpl{Context: context.Background(), Registry: &retryRegistry{}, Commands: commands}
	devices := []entities.DeviceSnapshot{{ID: "device", UnitID: "unit", State: "active"}}
	job := &simulationJob{State: entities.SimulationState{Seeding: true, Total: days * 4}}
	service.seed("unit", job, devices, midnight, days)
	if job.State.Seeding || job.State.Completed != 240 || len(commands.readings) != 240 {
		t.Fatal("incomplete calendar history")
	}
	seen := map[string]bool{}
	previous := 0
	for _, reading := range commands.readings {
		if reading.MeasuredAt.Before(midnight.AddDate(0, 0, -days)) || !reading.MeasuredAt.Before(midnight) || reading.VolumeML <= 0 {
			t.Fatal("invalid history slot")
		}
		if reading.MeasuredAt.In(midnight.Location()).Month() == time.September {
			previous++
		}
		if seen[reading.ID] {
			t.Fatal("duplicate slot in a single generation")
		}
		seen[reading.ID] = true
	}
	if previous != 120 {
		t.Fatal("previous month must contain all 30 days")
	}
	service.seed("unit", &simulationJob{}, devices, midnight, days)
	for _, reading := range commands.readings[240:] {
		if !seen[reading.ID] {
			t.Fatal("repeating history must preserve reading IDs")
		}
	}
}

type retryRegistry struct{ failure error }

func (r *retryRegistry) Resolve(context.Context, string) (entities.DeviceSnapshot, error) {
	return entities.DeviceSnapshot{ID: "device", UnitID: "unit", State: "active"}, r.failure
}
func (r *retryRegistry) Authorize(context.Context, string, string) (entities.UnitAccess, error) {
	return entities.UnitAccess{}, nil
}
func (r *retryRegistry) Devices(context.Context, string, string) ([]entities.DeviceSnapshot, error) {
	return nil, nil
}

type retryCommands struct{ count int }

func (r *retryCommands) Handle(context.Context, commands.RecordReadingCommand) (bool, error) {
	r.count++
	return true, nil
}
func TestLiveGeneratorRecoversAfterTemporaryDependencyFailure(t *testing.T) {
	registry := &retryRegistry{failure: valueobjects.ErrUnavailable}
	commands := &retryCommands{}
	service := &SimulationCommandServiceImpl{Context: context.Background(), Registry: registry, Commands: commands}
	job := &simulationJob{State: entities.SimulationState{Running: true}, Epoch: 1}
	devices := []entities.DeviceSnapshot{{ID: "device", UnitID: "unit", State: "active"}}
	if !service.liveSample("unit", job, 1, devices) || !job.State.Running || job.State.Error == "" {
		t.Fatal("temporary failures must remain retryable")
	}
	registry.failure = nil
	if !service.liveSample("unit", job, 1, devices) || job.State.Error != "" || commands.count != 1 {
		t.Fatal("generation must recover and clear the warning")
	}
	registry.failure = valueobjects.ErrNotFound
	if !service.liveSample("unit", job, 1, devices) || commands.count != 1 {
		t.Fatal("a deleted device must not stop other generators")
	}
	registry.failure = valueobjects.ErrUnauthorized
	if service.liveSample("unit", job, 1, devices) || job.State.Running {
		t.Fatal("authorization failures must stop generation")
	}
}

func TestHistoricalGenerationIsIdempotent(t *testing.T) {
	at := time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC)
	first := generatedReading("00000000-0000-4000-8000-000000000002", at, false)
	again := generatedReading(first.DeviceID, at, false)
	if first != again || first.Validate(time.Now()) != nil {
		t.Fatal("history must be valid and reproducible")
	}
	if generatedReading(first.DeviceID, at.Add(time.Hour), false).ID == first.ID {
		t.Fatal("different slots need different ids")
	}
	if generatedReading(first.DeviceID, at, true).BootID == first.BootID {
		t.Fatal("history and live must not share sequence namespace")
	}
}

func TestValidationScenarioProducesRecentHighUsageAndContinuousLeak(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	rows := scenarioReadings("00000000-0000-4000-8000-000000000002", now, "demo")
	if len(rows) != 5 || rows[0].VolumeML != 500000 || rows[1].VolumeML != 0 {
		t.Fatal("demo must emit high usage, a reset, then a leak")
	}
	var total int64
	for _, row := range rows {
		total += row.VolumeML
		if row.Validate(now.Add(time.Minute)) != nil {
			t.Fatal("invalid demo reading")
		}
	}
	for i := 2; i < len(rows); i++ {
		if rows[i].VolumeML != 6000 || rows[i].MeasuredAt.Sub(now) != time.Duration(i-1)*10*time.Second {
			t.Fatal("leak must stream forward after previous data")
		}
	}
	if total != 518000 || rows[4].MeasuredAt.Sub(now) != 30*time.Second {
		t.Fatal("demo must emit 518 liters over 30 seconds")
	}
	if len(scenarioReadings(rows[0].DeviceID, now, "invalid")) != 0 {
		t.Fatal("unknown scenario must not generate data")
	}
}
func TestLiveWaterFlowIncludesRealisticBurstsAndZeroIntervals(t *testing.T) {
	at := time.Date(2026, 10, 24, 18, 0, 10, 0, time.UTC)
	reading := generatedReading("00000000-0000-4000-8000-000000000002", at, true)
	if reading.VolumeML < 200 || reading.VolumeML > 710 {
		t.Fatal("live burst outside expected range")
	}
	if generatedReading(reading.DeviceID, at.Add(20*time.Second), true).VolumeML != 0 {
		t.Fatal("normal activity must include stops")
	}
}

func TestCompletedSimulationJobsReleaseCapacity(t *testing.T) {
	service := &SimulationCommandServiceImpl{jobs: map[string]*simulationJob{}, demoJobs: map[string]bool{}}
	for i := 0; i < 100; i++ {
		service.jobs[fmt.Sprint(i)] = &simulationJob{LastUsed: time.Unix(int64(i), 0)}
	}
	service.jobs["0"].State.Running = true
	service.jobs["1"].State.Seeding = true
	service.demoJobs["2"] = true
	job, err := service.jobFor("new")
	if err != nil || job == nil || len(service.jobs) != 100 || service.jobs["3"] != nil {
		t.Fatal("expected the oldest inactive unit to be reclaimed")
	}
	if service.jobs["0"] == nil || service.jobs["1"] == nil || service.jobs["2"] == nil {
		t.Fatal("active work was evicted")
	}
}
func TestActiveSimulationJobsCannotBeEvicted(t *testing.T) {
	service := &SimulationCommandServiceImpl{jobs: map[string]*simulationJob{}}
	for i := 0; i < 100; i++ {
		service.jobs[fmt.Sprint(i)] = &simulationJob{State: entities.SimulationState{Running: true}}
	}
	if _, err := service.jobFor("new"); err != valueobjects.ErrConflict {
		t.Fatal("active capacity must be preserved")
	}
}
func TestGeneratedGeneralIncludesAllZoneVolumes(t *testing.T) {
	at := time.Date(2026, 11, 15, 12, 0, 10, 0, time.UTC)
	general := entities.DeviceSnapshot{ID: "general", Category: "GENERAL"}
	for _, live := range []bool{true, false} {
		total := generatedMeterReading("unit", general, at, live).VolumeML
		var zones int64
		for i := 0; i < 11; i++ {
			zones += generatedMeterReading("unit", entities.DeviceSnapshot{ID: fmt.Sprint(i), Category: "BAÑO"}, at, live).VolumeML
		}
		if total <= 0 || zones > total {
			t.Fatal("zone measurements exceed the General")
		}
	}
}

type seededHistory struct {
	services.ReadingRepository
	rows []aggregates.RecordedReading
}

func (h seededHistory) History(context.Context, string, time.Time, time.Time, int) ([]aggregates.RecordedReading, error) {
	return h.rows, nil
}

func TestSeedingPreservesSamplesCreatedByOlderGenerators(t *testing.T) {
	at := time.Date(2026, 11, 1, 11, 0, 0, 0, time.UTC)
	original := generatedReading("device", at, false)
	original.VolumeML = 99999
	commands := &retryCommands{}
	service := &SimulationCommandServiceImpl{
		Context: context.Background(), Registry: &retryRegistry{}, Commands: commands,
		Readings: seededHistory{rows: []aggregates.RecordedReading{{Reading: original, UnitID: "unit"}}},
	}
	if err := service.record("unit", entities.DeviceSnapshot{ID: "device", Category: "GENERAL"}, at, false); err != nil || commands.count != 0 {
		t.Fatal("legacy history should be retained without a conflicting write")
	}
}
