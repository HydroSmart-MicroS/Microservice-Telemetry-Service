package services

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
)

type ReadingCommandService interface {
	Handle(context.Context, commands.RecordReadingCommand) (bool, error)
}
