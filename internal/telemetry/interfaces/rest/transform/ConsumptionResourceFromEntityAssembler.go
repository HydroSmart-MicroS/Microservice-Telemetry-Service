package transform

import (
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"hydrosmart/telemetry/internal/telemetry/interfaces/rest/resources"
)

func ToResource(unitID string, result services.ConsumptionResult) resources.ConsumptionResource {
	items := make([]resources.ReadingResource, 0, len(result.Readings))
	for _, r := range result.Readings {
		items = append(items, resources.ReadingResource{ReadingID: r.ID, DeviceID: r.DeviceID, UnitID: r.UnitID, VolumeML: r.VolumeML, IntervalSeconds: r.IntervalSeconds, MeasuredAt: r.MeasuredAt})
	}
	daily := make([]resources.DailyResource, 0, len(result.Summary.Daily))
	for _, d := range result.Summary.Daily {
		daily = append(daily, resources.DailyResource{Date: d.Date, DeviceID: d.DeviceID, Liters: float64(d.VolumeML) / 1000, ReadingCount: d.Count})
	}
	devices := make([]resources.DeviceConsumptionResource, 0, len(result.Summary.Devices))
	for _, d := range result.Summary.Devices {
		devices = append(devices, resources.DeviceConsumptionResource{DeviceID: d.DeviceID, Liters: float64(d.VolumeML) / 1000, LastReadingAt: d.LastReadingAt})
	}
	return resources.ConsumptionResource{UnitID: unitID, VolumeML: result.Summary.VolumeML, Liters: float64(result.Summary.VolumeML) / 1000, ReadingCount: result.Summary.ReadingCount, From: result.From, To: result.To, Items: items, Daily: daily, Devices: devices, FirstReadingAt: result.Summary.FirstReadingAt, LastReadingAt: result.Summary.LastReadingAt}
}
