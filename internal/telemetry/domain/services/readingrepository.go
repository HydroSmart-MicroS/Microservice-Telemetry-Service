package services

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"time"
)

type ReadingRepository interface {
	Insert(context.Context, aggregates.RecordedReading) (bool, error)
	Summary(context.Context, string, time.Time, time.Time) (aggregates.Consumption, error)
	History(context.Context, string, time.Time, time.Time, int) ([]aggregates.RecordedReading, error)
	Pending(context.Context) ([]aggregates.RecordedReading, error)
	Published(context.Context, string) error
}
