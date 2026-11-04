package application

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/application/internal/commandservices"
	"hydrosmart/telemetry/internal/telemetry/application/internal/eventhandlers"
	"hydrosmart/telemetry/internal/telemetry/application/internal/outboundservices/events"
	"hydrosmart/telemetry/internal/telemetry/application/internal/queryservices"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
)

func NewSimulation(ctx context.Context, commands services.ReadingCommandService, registry services.SimulationRegistry) services.SimulationService {
	return &commandservices.SimulationCommandServiceImpl{Context: ctx, Commands: commands, Registry: registry}
}

func NewReadingCommands(repository services.ReadingRepository, registry services.DeviceRegistry, gate services.ReadingLifecycle) services.ReadingCommandService {
	return commandservices.ReadingCommandServiceImpl{Repository: repository, Registry: registry, Gate: gate}
}
func NewConsumptionQueries(repository services.ReadingRepository, registry services.DeviceRegistry) services.ConsumptionQueryService {
	return queryservices.ConsumptionQueryServiceImpl{Repository: repository, Registry: registry}
}
func RunOutbox(ctx context.Context, repository services.ReadingRepository, publisher events.EventPublisher) {
	eventhandlers.RunOutbox(ctx, repository, publisher)
}
