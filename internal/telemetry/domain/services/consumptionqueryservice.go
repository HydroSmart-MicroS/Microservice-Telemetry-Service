package services

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/queries"
	"time"
)

type ConsumptionResult struct {
	Summary  aggregates.Consumption
	Readings []aggregates.RecordedReading
	From, To time.Time
}
type ConsumptionQueryService interface {
	Handle(context.Context, queries.GetConsumptionQuery, bool) (ConsumptionResult, error)
}
