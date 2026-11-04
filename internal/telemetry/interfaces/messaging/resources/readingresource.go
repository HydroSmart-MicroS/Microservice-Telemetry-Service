package resources

import "time"

type ReadingResource struct {
	Version         int       `json:"schema_version"`
	ID              string    `json:"reading_id"`
	DeviceID        string    `json:"device_id"`
	BootID          string    `json:"boot_id"`
	Sequence        *int64    `json:"sequence"`
	VolumeML        *int64    `json:"volume_ml"`
	IntervalSeconds *int64    `json:"interval_seconds"`
	MeasuredAt      time.Time `json:"measured_at"`
}
