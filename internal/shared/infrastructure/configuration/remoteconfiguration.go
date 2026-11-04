package configuration

import (
	"encoding/json"
	"fmt"
	"hydrosmart/telemetry/internal/shared/infrastructure/messaging"
	"net/http"
	"os"
	"strings"
	"time"
)

func Load() error {
	base := os.Getenv("CONFIG_SERVICE_URL")
	if base == "" {
		return nil
	}
	client := &http.Client{Timeout: 12 * time.Second, Transport: messaging.NewTransport()}
	res, err := client.Get(strings.TrimRight(base, "/") + "/v1/config/telemetry/local")
	if err != nil {
		return fmt.Errorf("Config Service unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("Config Service returned %d", res.StatusCode)
	}
	var values map[string]string
	if json.NewDecoder(res.Body).Decode(&values) != nil {
		return fmt.Errorf("invalid remote configuration")
	}
	allowed := map[string]bool{"MONGODB_DATABASE": true, "JWT_ISSUER": true, "JWKS_URL": true, "JWT_AUDIENCE": true, "PROPERTIES_INTERNAL_URL": true, "MQTT_BROKER_URL": true}
	if len(values) != len(allowed) {
		return fmt.Errorf("invalid remote configuration")
	}
	for key, value := range values {
		if !allowed[key] || strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("invalid remote configuration")
		}
		if _, ok := os.LookupEnv(key); !ok {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}
