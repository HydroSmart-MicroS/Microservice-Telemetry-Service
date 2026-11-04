package resources

import "time"

type ReadingResource struct {
	ReadingID       string    `json:"readingId"`
	DeviceID        string    `json:"deviceId"`
	UnitID          string    `json:"unitId"`
	VolumeML        int64     `json:"volumeMl"`
	IntervalSeconds int64     `json:"intervalSeconds"`
	MeasuredAt      time.Time `json:"measuredAt"`
}
type ConsumptionResource struct {
	FirstReadingAt *time.Time                  `json:"firstReadingAt"`
	LastReadingAt  *time.Time                  `json:"lastReadingAt"`
	Daily          []DailyResource             `json:"daily"`
	Devices        []DeviceConsumptionResource `json:"devices"`
	UnitID         string                      `json:"unitId"`
	VolumeML       int64                       `json:"volumeMl"`
	Liters         float64                     `json:"liters"`
	ReadingCount   int64                       `json:"readingCount"`
	From           time.Time                   `json:"from"`
	To             time.Time                   `json:"to"`
	Items          []ReadingResource           `json:"items"`
}
type DailyResource struct {
	Date         string  `json:"date"`
	DeviceID     string  `json:"deviceId"`
	Liters       float64 `json:"liters"`
	ReadingCount int64   `json:"readingCount"`
}
type DeviceConsumptionResource struct {
	DeviceID      string    `json:"deviceId"`
	Liters        float64   `json:"liters"`
	LastReadingAt time.Time `json:"lastReadingAt"`
}
