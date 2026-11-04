package eventhandlers

import (
	"context"
	"hydrosmart/telemetry/internal/telemetry/application/internal/outboundservices/events"
	"hydrosmart/telemetry/internal/telemetry/domain/services"
	"log"
	"time"
)

func RunOutbox(ctx context.Context, repository services.ReadingRepository, publisher events.EventPublisher) {
	defer publisher.Close()
	for {
		run, cancel := context.WithTimeout(ctx, 15*time.Second)
		pending, err := repository.Pending(run)
		if err == nil {
			for _, reading := range pending {
				if err = publisher.Publish(run, reading); err != nil {
					break
				}
				if err = repository.Published(run, reading.ID); err != nil {
					break
				}
			}
		}
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Print("Outbox delivery delayed; pending event retained")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
