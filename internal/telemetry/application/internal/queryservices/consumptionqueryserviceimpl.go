package queryservices

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/queries"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"time"
)

type ConsumptionResult = services.ConsumptionResult

type ConsumptionQueryServiceImpl struct {
	Repository services.ReadingRepository
	Registry   services.DeviceRegistry
}

func (s ConsumptionQueryServiceImpl) Handle(ctx context.Context, q queries.GetConsumptionQuery, history bool) (ConsumptionResult, error) {
	result := ConsumptionResult{From: q.From, To: q.To, Readings: []aggregates.RecordedReading{}}
	if !valueobjects.ValidID(q.UnitID) || !q.From.Before(q.To) || q.To.Sub(q.From) > 366*24*time.Hour || q.Limit < 1 || q.Limit > 500 {
		return result, valueobjects.ErrInvalid
	}
	access, err := s.Registry.Authorize(ctx, q.UnitID, q.Token)
	if err != nil {
		return result, err
	}
	if access.VisibleFrom != nil && result.From.Before(*access.VisibleFrom) {
		result.From = *access.VisibleFrom
	}
	if !result.From.Before(q.To) {
		return result, nil
	}
	result.Summary, err = s.Repository.Summary(ctx, q.UnitID, result.From, q.To)
	if err != nil {
		return result, err
	}
	if history {
		result.Readings, err = s.Repository.History(ctx, q.UnitID, result.From, q.To, q.Limit)
	}
	return result, err
}
