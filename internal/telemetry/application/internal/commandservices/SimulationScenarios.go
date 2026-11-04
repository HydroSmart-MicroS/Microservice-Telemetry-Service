package commandservices

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"time"
)

func (s *SimulationCommandServiceImpl) Scenario(ctx context.Context, unit, token, kind string) (entities.SimulationState, error) {
	if err := s.authorize(ctx, unit, token); err != nil {
		return entities.SimulationState{}, err
	}
	if kind != "high" && kind != "leak" && kind != "normal" && kind != "demo" {
		return entities.SimulationState{}, valueobjects.ErrInvalid
	}
	devices, err := s.Registry.Devices(ctx, unit, token)
	if err != nil {
		return entities.SimulationState{}, err
	}
	now := time.Now().UTC()
	for _, device := range devices {
		if device.UnitID != unit || device.State != "active" {
			continue
		}
		if kind == "demo" {
			if err := s.startDemo(unit, device.ID, now); err != nil {
				return entities.SimulationState{}, err
			}
			return s.Status(ctx, unit, token)
		}
		for _, reading := range scenarioReadings(device.ID, now, kind) {
			_, err = s.Commands.Handle(ctx, commands.RecordReadingCommand{Topic: "hydrosmart/v1/devices/" + device.ID + "/readings", Reading: reading})
			if err != nil {
				return entities.SimulationState{}, err
			}
		}
		return s.Status(ctx, unit, token)
	}
	return entities.SimulationState{}, valueobjects.ErrInvalid
}

func scenarioReadings(device string, now time.Time, kind string) []aggregates.Reading {
	count := 1
	switch kind {
	case "leak":
		count = 18
	case "demo":
		count = 5
	case "high", "normal":
	default:
		return nil
	}
	result := make([]aggregates.Reading, 0, count)
	for i := 0; i < count; i++ {
		reading := generatedReading(device, now.Add(time.Duration(i-count+1)*10*time.Second), true)
		switch kind {
		case "high":
			reading.VolumeML = 500000
		case "leak":
			reading.VolumeML = 1000
		case "normal":
			reading.VolumeML = 0
		case "demo":
			at := now.Add(time.Duration(i-1) * 10 * time.Second)
			if i == 0 {
				at = now
			}
			if i == 1 {
				at = now.Add(time.Millisecond)
			}
			reading = generatedReading(device, at, true)
			reading.VolumeML = 6000
			if i == 0 {
				reading.VolumeML = 500000
			}
			if i == 1 {
				reading.VolumeML = 0
			}
		}
		result = append(result, reading)
	}
	return result
}

// Stream the demonstration forward so earlier zero readings cannot interrupt its leak window.
func (s *SimulationCommandServiceImpl) startDemo(unit, device string, at time.Time) error {
	s.mu.Lock()
	if s.demoJobs == nil {
		s.demoJobs = map[string]bool{}
	}
	if s.jobs == nil {
		s.jobs = map[string]*simulationJob{}
	}
	if s.demoJobs[unit] || len(s.demoJobs) >= 10 {
		s.mu.Unlock()
		return valueobjects.ErrConflict
	}
	job := s.jobs[unit]
	if job == nil {
		if len(s.jobs) >= 100 {
			s.mu.Unlock()
			return valueobjects.ErrConflict
		}
		job = &simulationJob{}
		s.jobs[unit] = job
	}
	job.State.Error = ""
	epoch := job.Epoch
	s.demoJobs[unit] = true
	s.mu.Unlock()
	go func() {
		defer func() { s.mu.Lock(); delete(s.demoJobs, unit); s.mu.Unlock() }()
		for _, reading := range scenarioReadings(device, at, "demo") {
			timer := time.NewTimer(time.Until(reading.MeasuredAt))
			select {
			case <-s.Context.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			s.mu.Lock()
			valid := job.Epoch == epoch
			s.mu.Unlock()
			if !valid {
				return
			}
			ctx, cancel := context.WithTimeout(s.Context, 8*time.Second)
			actual, err := s.Registry.Resolve(ctx, device)
			if err == nil && (actual.State != "active" || actual.UnitID != unit) {
				cancel()
				return
			}
			if err == nil {
				_, err = s.Commands.Handle(ctx, commands.RecordReadingCommand{Topic: "hydrosmart/v1/devices/" + device + "/readings", Reading: reading})
			}
			cancel()
			if err != nil {
				s.mu.Lock()
				job.State.Error = "No se pudo completar la prueba de alertas. Revisa la conexión y vuelve a intentarlo."
				s.mu.Unlock()
				return
			}
		}
	}()
	return nil
}
