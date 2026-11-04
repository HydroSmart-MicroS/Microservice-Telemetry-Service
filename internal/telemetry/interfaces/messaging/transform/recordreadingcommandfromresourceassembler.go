package transform

import (
	"bytes"
	"encoding/json"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/interfaces/messaging/resources"
	"io"
)

func ToCommand(topic string, payload []byte) (commands.RecordReadingCommand, error) {
	var dto resources.ReadingResource
	var command commands.RecordReadingCommand
	if len(payload) > 16384 {
		return command, valueobjects.ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&dto) != nil || decoder.Decode(new(any)) != io.EOF || dto.Version != 1 || dto.Sequence == nil || dto.VolumeML == nil || dto.IntervalSeconds == nil {
		return command, valueobjects.ErrInvalid
	}
	command = commands.RecordReadingCommand{Topic: topic, Reading: aggregates.Reading{ID: dto.ID, DeviceID: dto.DeviceID, BootID: dto.BootID, Sequence: *dto.Sequence, VolumeML: *dto.VolumeML, IntervalSeconds: *dto.IntervalSeconds, MeasuredAt: dto.MeasuredAt}}
	return command, nil
}
