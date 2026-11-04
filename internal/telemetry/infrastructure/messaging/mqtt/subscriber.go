package mqtt

import (
	"context"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"hydrosmart/telemetry/internal/telemetry/interfaces/messaging"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Subscriber struct {
	Client mqtt.Client
	done   chan struct{}
	cancel context.CancelFunc
	stop   sync.Once
	ready  *atomic.Bool
}

func Start(ctx context.Context, broker, password string, handler messaging.ReadingMessageHandler) (*Subscriber, error) {
	workerCtx, cancel := context.WithCancel(ctx)
	messages := make(chan mqtt.Message, 100)
	done := make(chan struct{})
	ready := &atomic.Bool{}
	go func() {
		defer close(done)
		for {
			select {
			case <-workerCtx.Done():
				return
			case m := <-messages:
				handler.Handle(workerCtx, m)
			}
		}
	}()
	options := mqtt.NewClientOptions().AddBroker(broker).SetUsername("ingestor").SetPassword(password).
		SetClientID("hydrosmart-telemetry-ingestor").SetProtocolVersion(4).SetCleanSession(false).
		SetAutoReconnect(true).SetAutoAckDisabled(true).SetConnectTimeout(5 * time.Second).SetKeepAlive(20 * time.Second)
	options.SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
		select {
		case messages <- m:
		case <-workerCtx.Done():
		}
	})
	options.OnConnect = func(c mqtt.Client) {
		token := c.Subscribe("hydrosmart/v1/devices/+/readings", 1, nil)
		if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
			log.Print("MQTT subscription unavailable")
		} else {
			ready.Store(true)
			log.Print("MQTT reading subscription ready")
		}
	}
	options.OnConnectionLost = func(c mqtt.Client, err error) {
		ready.Store(false)
		log.Print("MQTT connection interrupted; reconnecting")
	}
	client := mqtt.NewClient(options)
	token := client.Connect()
	if !token.WaitTimeout(7*time.Second) || token.Error() != nil {
		cancel()
		client.Disconnect(0)
		return nil, fmt.Errorf("MQTT unavailable")
	}
	return &Subscriber{Client: client, done: done, cancel: cancel, ready: ready}, nil
}
func (s *Subscriber) Ready() bool { return s.Client.IsConnectionOpen() && s.ready.Load() }
func (s *Subscriber) Close() {
	s.stop.Do(func() {
		s.cancel()
		s.Client.Disconnect(250)
		select {
		case <-s.done:
		case <-time.After(time.Second):
		}
	})
}
