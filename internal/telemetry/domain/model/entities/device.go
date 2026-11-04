package entities

import "time"

type DeviceSnapshot struct{ ID, UnitID, State string }
type UnitAccess struct{ VisibleFrom *time.Time }
