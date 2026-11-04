package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"
)

type traceKey struct{}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)
var pathIdentity = regexp.MustCompile(`/[A-Za-z0-9_-]{20,}`)

func traceID(value string) string {
	if uuidPattern.MatchString(value) {
		return value
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 0x40
	b[8] = (b[8] & 63) | 0x80
	v := hex.EncodeToString(b[:])
	return v[:8] + "-" + v[8:12] + "-" + v[12:16] + "-" + v[16:20] + "-" + v[20:]
}
func requestID(ctx context.Context) string {
	value, _ := ctx.Value(traceKey{}).(string)
	return traceID(value)
}

type tracedWriter struct {
	http.ResponseWriter
	status int
}

func (w *tracedWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *tracedWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}
func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := traceID(r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Request-ID", id)
		start := time.Now()
		out := &tracedWriter{ResponseWriter: w}
		defer func() {
			status := out.status
			if status == 0 {
				status = 200
			}
			body, _ := json.Marshal(map[string]any{"event": "http_request", "service": "telemetry", "requestId": id, "method": r.Method, "path": pathIdentity.ReplaceAllString(r.URL.Path, "/:id"), "status": status, "durationMs": float64(time.Since(start).Microseconds()) / 1000})
			log.Print(string(body))
		}()
		next.ServeHTTP(out, r.WithContext(context.WithValue(r.Context(), traceKey{}, id)))
	})
}
