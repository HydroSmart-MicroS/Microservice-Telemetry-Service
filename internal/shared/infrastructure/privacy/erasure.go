package privacy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"net/http"
	"strings"
	"sync"
)

// shortcut: a single local telemetry instance; coordinate this gate before horizontal scaling.
var Gate sync.RWMutex

type readKey struct{}
type ReadingGuard struct{}

func (ReadingGuard) Enter(ctx context.Context) func() {
	if ctx.Value(readKey{}) == true {
		return func() {}
	}
	Gate.RLock()
	return Gate.RUnlock
}
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func Handler(collection *mongo.Collection, secret string, forget func([]string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(secret) < 32 || subtle.ConstantTimeCompare([]byte(secret), []byte(r.Header.Get("X-Service-Token"))) != 1 {
			w.WriteHeader(401)
			return
		}
		var body struct {
			UnitIDs []string `json:"unitIds"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil || body.UnitIDs == nil || len(body.UnitIDs) > 10000 {
			w.WriteHeader(400)
			return
		}
		for _, unit := range body.UnitIDs {
			if !valueobjects.ValidID(unit) {
				w.WriteHeader(400)
				return
			}
		}
		Gate.Lock()
		defer Gate.Unlock()
		forget(body.UnitIDs)
		tombstones := collection.Database().Collection("erased_units")
		for _, unit := range body.UnitIDs {
			if _, err := tombstones.UpdateOne(r.Context(), bson.M{"_id": Hash(unit)}, bson.M{"$set": bson.M{"erased": true}}, options.UpdateOne().SetUpsert(true)); err != nil {
				w.WriteHeader(503)
				return
			}
			if _, err := collection.DeleteMany(r.Context(), bson.M{"unit_id": unit}); err != nil {
				w.WriteHeader(503)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"erased":true}`))
	}
}
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
		if user {
			Gate.RLock()
			defer Gate.RUnlock()
			r = r.WithContext(context.WithValue(r.Context(), readKey{}, true))
		}
		next.ServeHTTP(w, r)
	})
}
