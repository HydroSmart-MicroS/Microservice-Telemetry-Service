package messaging

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/domain/model/commands"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"testing"
)

type stubCommands struct {
	err    error
	called bool
}

func (s *stubCommands) Handle(ctx context.Context, c commands.RecordReadingCommand) (bool, error) {
	s.called = true
	return s.err == nil, s.err
}

type testMessage struct{ ack bool }

func (m *testMessage) Duplicate() bool { return false }
func (m *testMessage) Qos() byte       { return 1 }
func (m *testMessage) Retained() bool  { return false }
func (m *testMessage) Topic() string {
	return "hydrosmart/v1/devices/00000000-0000-4000-8000-000000000002/readings"
}
func (m *testMessage) MessageID() uint16 { return 1 }
func (m *testMessage) Payload() []byte {
	return []byte(`{"schema_version":1,"reading_id":"00000000-0000-4000-8000-000000000001","device_id":"00000000-0000-4000-8000-000000000002","boot_id":"00000000-0000-4000-8000-000000000003","sequence":1,"volume_ml":1250,"interval_seconds":60,"measured_at":"2026-10-24T00:00:00Z"}`)
}
func (m *testMessage) Ack() { m.ack = true }
func TestAcknowledgesOnlyAfterPersistenceOrPermanentRejection(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		ack  bool
	}{
		{"persisted", nil, true}, {"permanent", valueobjects.ErrInvalid, true}, {"dependency_failure", valueobjects.ErrUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			command := &stubCommands{err: tc.err}
			message := &testMessage{}
			ReadingMessageHandler{Commands: command}.Handle(ctx, message)
			if !command.called || message.ack != tc.ack {
				t.Fatalf("called=%v ack=%v", command.called, message.ack)
			}
		})
	}
}
