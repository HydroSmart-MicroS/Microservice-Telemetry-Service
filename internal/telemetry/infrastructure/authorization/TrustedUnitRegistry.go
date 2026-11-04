package authorization

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/entities"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
)

// Used only by the service-token-protected internal consumption endpoint.
type TrustedUnitRegistry struct{ services.DeviceRegistry }

func (t TrustedUnitRegistry) Authorize(context.Context, string, string) (entities.UnitAccess, error) {
	return entities.UnitAccess{}, nil
}
