package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"log"
	"time"
)

type ReadingEventPublisher struct {
	URL        string
	connection *amqp.Connection
	channel    *amqp.Channel
	returned   chan amqp.Return
}

func (p *ReadingEventPublisher) Close() {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.connection != nil {
		_ = p.connection.Close()
	}
	p.channel = nil
	p.connection = nil
}
func (p *ReadingEventPublisher) connect() error {
	if p.channel != nil && !p.channel.IsClosed() {
		return nil
	}
	p.Close()
	connection, err := amqp.DialConfig(p.URL, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return fmt.Errorf("RabbitMQ unavailable")
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return err
	}
	p.connection = connection
	p.channel = channel
	if err = channel.ExchangeDeclare("hydrosmart.events", "topic", true, false, false, false, nil); err != nil {
		p.Close()
		return err
	}
	if err = channel.Confirm(false); err != nil {
		p.Close()
		return err
	}
	p.returned = make(chan amqp.Return, 1)
	channel.NotifyReturn(p.returned)
	return nil
}
func (p *ReadingEventPublisher) Publish(ctx context.Context, r aggregates.RecordedReading) error {
	if err := p.connect(); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"event_id": r.ID, "event_type": "telemetry.reading.recorded.v1", "occurred_at": r.ReceivedAt.UTC().Format(time.RFC3339Nano), "producer": "telemetry",
		"data": map[string]any{"reading_id": r.ID, "device_id": r.DeviceID, "unit_id": r.UnitID, "volume_ml": r.VolumeML, "interval_seconds": r.IntervalSeconds, "measured_at": r.MeasuredAt.UTC().Format(time.RFC3339Nano)},
	})
	if err != nil {
		return err
	}
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(ctx, "hydrosmart.events", "telemetry.reading.recorded.v1", true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: r.ID, Timestamp: r.ReceivedAt, Body: payload,
	})
	if err != nil {
		p.Close()
		return err
	}
	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil || !confirmed {
		p.Close()
		return fmt.Errorf("event not confirmed")
	}
	select {
	case <-p.returned:
		p.Close()
		return fmt.Errorf("event unroutable")
	default:
		log.Printf(`{"event":"message_published","service":"telemetry","eventId":%q,"eventType":"telemetry.reading.recorded.v1"}`, r.ID)
		return nil
	}
}
