package services

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
)

type DeviceRegistry interface {
	Resolve(context.Context, string) (entities.DeviceSnapshot, error)
	Authorize(context.Context, string, string) (entities.UnitAccess, error)
}
