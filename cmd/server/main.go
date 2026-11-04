package main

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"hydrosmart/telemetry/internal/shared/infrastructure/configuration"
	"hydrosmart/telemetry/internal/shared/infrastructure/health"
	brokerrpc "hydrosmart/telemetry/internal/shared/infrastructure/messaging"
	"hydrosmart/telemetry/internal/shared/infrastructure/privacy"
	"hydrosmart/telemetry/internal/telemetry/application"
	"hydrosmart/telemetry/internal/telemetry/infrastructure/authorization"
	mqttadapter "hydrosmart/telemetry/internal/telemetry/infrastructure/messaging/mqtt"
	"hydrosmart/telemetry/internal/telemetry/infrastructure/messaging/rabbitmq"
	"hydrosmart/telemetry/internal/telemetry/infrastructure/persistence/mongodb/repositories"
	tokenservice "hydrosmart/telemetry/internal/telemetry/infrastructure/tokens/jwt/services"
	"hydrosmart/telemetry/internal/telemetry/interfaces/acl"
	"hydrosmart/telemetry/internal/telemetry/interfaces/messaging"
	"hydrosmart/telemetry/internal/telemetry/interfaces/rest"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Missing configuration: %s", key)
	}
	return value
}
func main() {
	if err := configuration.Load(); err != nil {
		log.Fatal(err)
	}
	if os.Getenv("CONFIG_SERVICE_URL") != "" {
		log.Print("Loaded telemetry configuration from Config Service")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := mongo.Connect(options.Client().ApplyURI(required("MONGODB_URI")).SetServerSelectionTimeout(3 * time.Second))
	if err != nil {
		log.Fatal("Invalid MongoDB configuration")
	}
	defer client.Disconnect(context.Background())
	repository := repositories.MongoReadingRepository{Collection: client.Database(required("MONGODB_DATABASE")).Collection("readings")}
	setup, cancel := context.WithTimeout(ctx, 15*time.Second)
	err = repository.EnsureIndexes(setup)
	cancel()
	if err != nil {
		log.Fatal("MongoDB indexes unavailable")
	}
	httpClient := &http.Client{Timeout: 12 * time.Second, Transport: brokerrpc.NewTransport()}
	registry := acl.PropertiesContextFacade{BaseURL: required("PROPERTIES_INTERNAL_URL"), ServiceToken: required("INTERNAL_SERVICE_TOKEN"), Client: httpClient}
	commands := application.NewReadingCommands(repository, registry, privacy.ReadingGuard{})
	subscriber, err := mqttadapter.Start(ctx, required("MQTT_BROKER_URL"), required("MQTT_INGESTOR_PASSWORD"), messaging.ReadingMessageHandler{Commands: commands})
	if err != nil {
		log.Fatal("MQTT unavailable")
	}
	defer subscriber.Close()
	publisher := &rabbitmq.ReadingEventPublisher{URL: required("RABBITMQ_URL")}
	outboxDone := make(chan struct{})
	go func() { defer close(outboxDone); application.RunOutbox(ctx, repository, publisher) }()
	tokens := &tokenservice.JwtTokenService{URL: required("JWKS_URL"), Issuer: required("JWT_ISSUER"), Audience: required("JWT_AUDIENCE"), Client: httpClient}
	controller := rest.ConsumptionController{Queries: application.NewConsumptionQueries(repository, registry), Tokens: tokens}
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/consumption", controller)
	mux.Handle("GET /api/v1/consumption/readings", controller)
	mux.Handle("GET /api/v1/consumption/daily", controller)
	mux.Handle("GET /internal/v1/consumption", rest.InternalConsumptionController{Queries: application.NewConsumptionQueries(repository, authorization.TrustedUnitRegistry{DeviceRegistry: registry}), Secret: required("INTERNAL_SERVICE_TOKEN")})
	generator := application.NewSimulation(ctx, commands, registry)
	simulation := rest.SimulationController{Service: generator, Tokens: tokens}
	mux.Handle("POST /internal/v1/privacy/{user}/erase", privacy.Handler(repository.Collection, required("INTERNAL_SERVICE_TOKEN"), func(units []string) { generator.(interface{ Forget([]string) }).Forget(units) }))
	mux.Handle("GET /api/v1/simulation/{unitID}", simulation)
	mux.Handle("POST /api/v1/simulation/{unitID}", simulation)

	var rpcState atomic.Pointer[brokerrpc.Server]
	healthController := health.Controller{Service: "telemetry", Checks: map[string]health.Probe{
		"database": func(ctx context.Context) (map[string]any, error) {
			return map[string]any{"type": "MongoDB", "check": "ping"}, client.Ping(ctx, readpref.Primary())
		},
		"mqtt": func(context.Context) (map[string]any, error) {
			if !subscriber.Ready() {
				return map[string]any{"subscription": "DOWN"}, fmt.Errorf("MQTT unavailable")
			}
			return map[string]any{"connection": "UP", "subscription": "UP"}, nil
		},
		"rabbitmq": func(ctx context.Context) (map[string]any, error) {
			state := rpcState.Load()
			return brokerrpc.BrokerHealth(ctx, "telemetry", state != nil && state.Ready())
		},
	}}
	mux.Handle("/health/", healthController)

	handler := brokerrpc.Trace(privacy.Middleware(mux))
	rpcState.Store(brokerrpc.StartServer(ctx, "telemetry", handler))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("telemetry listening on :%s", port)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print("HTTP server stopped")
		stop()
	}
	stop()
	select {
	case <-outboxDone:
	case <-time.After(5 * time.Second):
	}
}
