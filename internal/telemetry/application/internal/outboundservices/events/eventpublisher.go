package events

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
)

type EventPublisher interface {
	Publish(context.Context, aggregates.RecordedReading) error
	Close()
}
