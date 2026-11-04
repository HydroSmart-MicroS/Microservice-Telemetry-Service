package messaging

import (
	"context"
	"errors"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"hydrosmart/telemetry/internal/telemetry/interfaces/messaging/transform"
	"log"
	"time"
)

type ReadingMessageHandler struct {
	Commands services.ReadingCommandService
}

func (h ReadingMessageHandler) Handle(ctx context.Context, message mqtt.Message) {
	if message.Qos() != 1 || message.Retained() {
		message.Ack()
		return
	}
	command, err := transform.ToCommand(message.Topic(), message.Payload())
	if err != nil {
		log.Print("Rejected malformed MQTT reading")
		message.Ack()
		return
	}
	for {
		attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err = h.Commands.Handle(attempt, command)
		cancel()
		if err == nil {
			message.Ack()
			return
		}
		if errors.Is(err, valueobjects.ErrInvalid) || errors.Is(err, valueobjects.ErrConflict) || errors.Is(err, valueobjects.ErrNotFound) {
			log.Printf("Rejected MQTT reading: %v", err)
			message.Ack()
			return
		}
		log.Print("Reading persistence delayed; MQTT message remains unacknowledged")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
