package commands

import "hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"

type RecordReadingCommand struct {
	Topic   string
	Reading aggregates.Reading
}
