package commandservices

import (
	"context"

	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"strings"
	"time"
)

type ReadingCommandServiceImpl struct {
	Repository services.ReadingRepository
	Registry   services.DeviceRegistry
	Gate       services.ReadingLifecycle
}

func (s ReadingCommandServiceImpl) Handle(ctx context.Context, c commands.RecordReadingCommand) (bool, error) {
	if s.Gate != nil {
		unlock := s.Gate.Enter(ctx)
		defer unlock()
	}
	if err := c.Reading.Validate(time.Now()); err != nil {
		return false, err
	}
	expected := "hydrosmart/v1/devices/" + c.Reading.DeviceID + "/readings"
	if strings.TrimSpace(c.Topic) != expected {
		return false, valueobjects.ErrInvalid
	}
	device, err := s.Registry.Resolve(ctx, c.Reading.DeviceID)
	if err != nil {
		return false, err
	}
	if device.ID != c.Reading.DeviceID || !valueobjects.ValidID(device.UnitID) || device.State != "active" {
		return false, valueobjects.ErrInvalid
	}
	return s.Repository.Insert(ctx, aggregates.RecordedReading{Reading: c.Reading, UnitID: device.UnitID, ReceivedAt: time.Now().UTC()})
}
