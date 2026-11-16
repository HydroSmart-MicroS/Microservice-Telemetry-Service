package services

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
)

type SimulationRegistry interface {
	DeviceRegistry
}
type SimulationService interface {
	Scenario(context.Context, string, string, string) (entities.SimulationState, error)
	Status(context.Context, string, string) (entities.SimulationState, error)
	Handle(context.Context, string, string, string, int) (entities.SimulationState, error)
}
