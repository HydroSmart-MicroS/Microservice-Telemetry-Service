package aggregates

import (
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"time"
)

type Reading struct {
	ID, DeviceID, BootID string
	Sequence, VolumeML   int64
	IntervalSeconds      int64
	MeasuredAt           time.Time
}

func (r Reading) Validate(now time.Time) error {
	if !valueobjects.ValidID(r.ID) || !valueobjects.ValidID(r.DeviceID) || !valueobjects.ValidID(r.BootID) ||
		r.Sequence < 0 || r.VolumeML < 0 || r.IntervalSeconds < 1 || r.MeasuredAt.IsZero() ||
		r.MeasuredAt.After(now.Add(5*time.Minute)) {
		return valueobjects.ErrInvalid
	}
	return nil
}

type RecordedReading struct {
	Reading
	UnitID     string
	ReceivedAt time.Time
}
type DailyConsumption struct {
	Date, DeviceID  string
	VolumeML, Count int64
}
type DeviceConsumption struct {
	DeviceID      string
	VolumeML      int64
	LastReadingAt time.Time
}
type Consumption struct {
	VolumeML, ReadingCount        int64
	FirstReadingAt, LastReadingAt *time.Time
	Daily                         []DailyConsumption
	Devices                       []DeviceConsumption
}
