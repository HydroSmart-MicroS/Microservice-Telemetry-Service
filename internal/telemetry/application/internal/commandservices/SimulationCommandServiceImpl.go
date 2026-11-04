package commandservices

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"sync"
	"time"
)

type simulationJob struct {
	State   entities.SimulationState
	Devices []entities.DeviceSnapshot
	Epoch   uint64
}
type SimulationCommandServiceImpl struct {
	Context  context.Context
	Commands services.ReadingCommandService
	Registry services.SimulationRegistry
	mu       sync.Mutex
	// shortcut: running generators live in process memory; restart them from the panel after a service restart.
	jobs     map[string]*simulationJob
	demoJobs map[string]bool
}

func (s *SimulationCommandServiceImpl) authorize(ctx context.Context, unit, token string) error {
	if !valueobjects.ValidID(unit) {
		return valueobjects.ErrInvalid
	}
	access, err := s.Registry.Authorize(ctx, unit, token)
	if err != nil {
		return err
	}
	// Properties always supplies a tenancy start to tenants; an unrestricted access window belongs to the owner.
	if access.VisibleFrom != nil {
		return valueobjects.ErrForbidden
	}
	return nil
}
func (s *SimulationCommandServiceImpl) Status(ctx context.Context, unit, token string) (entities.SimulationState, error) {
	if err := s.authorize(ctx, unit, token); err != nil {
		return entities.SimulationState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if job := s.jobs[unit]; job != nil {
		return job.State, nil
	}
	return entities.SimulationState{}, nil
}
func (s *SimulationCommandServiceImpl) Handle(ctx context.Context, unit, token, action string, days int) (entities.SimulationState, error) {
	if err := s.authorize(ctx, unit, token); err != nil {
		return entities.SimulationState{}, err
	}
	if action != "seed" && action != "start" && action != "pause" {
		return entities.SimulationState{}, valueobjects.ErrInvalid
	}
	midnight, days := historyWindow(time.Now(), days)
	if action == "seed" && (days < 1 || days > 62) {
		return entities.SimulationState{}, valueobjects.ErrInvalid
	}
	var devices []entities.DeviceSnapshot
	if action != "pause" {
		all, err := s.Registry.Devices(ctx, unit, token)
		if err != nil {
			return entities.SimulationState{}, err
		}
		for _, d := range all {
			if d.State == "active" && d.UnitID == unit {
				devices = append(devices, d)
			}
		}
		if len(devices) == 0 || len(devices) > 12 {
			return entities.SimulationState{}, valueobjects.ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs == nil {
		s.jobs = map[string]*simulationJob{}
	}
	job := s.jobs[unit]
	if job == nil {
		if len(s.jobs) >= 100 {
			return entities.SimulationState{}, valueobjects.ErrConflict
		}
		job = &simulationJob{}
		s.jobs[unit] = job
	}
	if action == "pause" {
		job.State.Running = false
		job.State.Error = ""
		job.Epoch++
		return job.State, nil
	}
	active := 0
	for _, j := range s.jobs {
		if j.State.Running || j.State.Seeding {
			active++
		}
	}
	if active >= 10 && !job.State.Running && !job.State.Seeding {
		return job.State, valueobjects.ErrConflict
	}
	job.Devices = devices
	if action == "start" {
		if !job.State.Running {
			job.State.Running = true
			job.State.Error = ""
			job.Epoch++
			go s.live(unit, job, job.Epoch)
		}
	} else {
		if job.State.Seeding {
			return job.State, nil
		}
		job.State.Seeding = true
		job.State.Completed = 0
		job.State.Total = days * 4 * len(devices)
		job.State.Error = ""
		go s.seed(unit, job, devices, midnight, days)
	}
	return job.State, nil
}
func stableID(key string) string {
	sum := sha256.Sum256([]byte(key))
	sum[6] = (sum[6] & 15) | 0x50
	sum[8] = (sum[8] & 63) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
func generatedReading(device string, at time.Time, live bool) aggregates.Reading {
	kind := "history"
	interval := int64(21600)
	if live {
		kind = "live"
		interval = 10
	}
	at = at.UTC().Truncate(time.Millisecond)
	key := device + "/" + kind + "/" + at.Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(key))
	volume := int64(25000) + int64(sum[0])*150
	if live {
		volume = 200 + int64(sum[0])*2
		if at.Second() >= 20 {
			volume = 0
		}
	}
	sequence := at.Unix()
	if live {
		sequence = at.UnixMilli()
	}
	return aggregates.Reading{ID: stableID(key), DeviceID: device, BootID: stableID(device + "/simulation/" + kind), Sequence: sequence, VolumeML: volume, IntervalSeconds: interval, MeasuredAt: at}
}
func (s *SimulationCommandServiceImpl) record(unit string, device entities.DeviceSnapshot, at time.Time, live bool) error {
	ctx, cancel := context.WithTimeout(s.Context, 8*time.Second)
	defer cancel()
	// Recheck the registry for each sample, so disabling a device stops generation immediately.
	actual, err := s.Registry.Resolve(ctx, device.ID)
	if errors.Is(err, valueobjects.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if actual.UnitID != unit || actual.State != "active" {
		return nil
	}
	reading := generatedReading(device.ID, at, live)
	_, err = s.Commands.Handle(ctx, commands.RecordReadingCommand{Topic: "hydrosmart/v1/devices/" + device.ID + "/readings", Reading: reading})
	return err
}
func historyWindow(now time.Time, days int) (time.Time, int) {
	lima := time.FixedZone("America/Lima", -5*3600)
	now = now.In(lima)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, lima)
	if days == 0 {
		start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, lima)
		days = int(midnight.Sub(start) / (24 * time.Hour))
	}
	return midnight, days
}
func (s *SimulationCommandServiceImpl) seed(unit string, job *simulationJob, devices []entities.DeviceSnapshot, midnight time.Time, days int) {
	for day := days; day >= 1; day-- {
		for _, hour := range []int{6, 12, 18, 23} {
			for _, device := range devices {
				s.mu.Lock()
				active := job.State.Seeding
				s.mu.Unlock()
				if !active {
					return
				}
				if err := s.Context.Err(); err != nil {
					return
				}
				err := s.record(unit, device, midnight.AddDate(0, 0, -day).Add(time.Duration(hour)*time.Hour), false)
				s.mu.Lock()
				if err != nil {
					job.State.Error = "No se pudieron generar todas las lecturas. Vuelve a intentarlo."
					job.State.Seeding = false
					s.mu.Unlock()
					return
				}
				job.State.Completed++
				s.mu.Unlock()
			}
		}
	}
	s.mu.Lock()
	job.State.Seeding = false
	s.mu.Unlock()
}
func (s *SimulationCommandServiceImpl) liveSample(unit string, job *simulationJob, epoch uint64, devices []entities.DeviceSnapshot) bool {
	for _, device := range devices {
		if s.Context.Err() != nil {
			return false
		}
		err := s.record(unit, device, time.Now().UTC(), true)
		if err != nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			if job.Epoch != epoch {
				return false
			}
			if errors.Is(err, valueobjects.ErrInvalid) || errors.Is(err, valueobjects.ErrForbidden) || errors.Is(err, valueobjects.ErrUnauthorized) {
				job.State.Error = "La generación se detuvo porque el medidor ya no permite registrar lecturas. Revisa sus datos y permisos."
				job.State.Running = false
				return false
			}
			job.State.Error = "Conexión temporalmente interrumpida. Reintentando automáticamente…"
			return true
		}
	}
	s.mu.Lock()
	if job.Epoch == epoch {
		job.State.Error = ""
	}
	s.mu.Unlock()
	return true
}
func (s *SimulationCommandServiceImpl) live(unit string, job *simulationJob, epoch uint64) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		running := job.State.Running && job.Epoch == epoch
		devices := append([]entities.DeviceSnapshot(nil), job.Devices...)
		s.mu.Unlock()
		if !running {
			return
		}
		if !s.liveSample(unit, job, epoch, devices) {
			return
		}
		select {
		case <-s.Context.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *SimulationCommandServiceImpl) Forget(units []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, unit := range units {
		if job := s.jobs[unit]; job != nil {
			job.State.Running = false
			job.State.Seeding = false
			job.Epoch++
			delete(s.jobs, unit)
		}
		delete(s.demoJobs, unit)
	}
}
