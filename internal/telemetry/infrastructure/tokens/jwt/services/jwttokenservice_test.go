package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignatureIssuerAudienceAndExpiration(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var active atomic.Bool
	active.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/internal/v1/users/") {
			if !active.Load() {
				w.WriteHeader(404)
			}
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": "test", "x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32))),
		}}})
	}))
	defer server.Close()
	verifier := &JwtTokenService{URL: server.URL, Issuer: "http://issuer", Audience: "hydrosmart-api", Client: server.Client()}
	sign := func(issuer, audience string, expires time.Time, method jwt.SigningMethod) string {
		token := jwt.NewWithClaims(method, jwt.MapClaims{"sub": "user", "profile_type": "propietario", "iss": issuer, "aud": audience, "exp": expires.Unix()})
		token.Header["kid"] = "test"
		var signingKey any = key
		if method.Alg() == "HS256" {
			signingKey = []byte("not-an-ecdsa-key")
		}
		raw, e := token.SignedString(signingKey)
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	valid := sign("http://issuer", "hydrosmart-api", time.Now().Add(time.Minute), jwt.SigningMethodES256)
	if err := verifier.Verify(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	active.Store(false)
	if verifier.Verify(context.Background(), valid) == nil {
		t.Fatal("deleted account token accepted")
	}
	active.Store(true)
	for _, raw := range []string{
		valid + "tampered",
		sign("http://other", "hydrosmart-api", time.Now().Add(time.Minute), jwt.SigningMethodES256),
		sign("http://issuer", "wrong", time.Now().Add(time.Minute), jwt.SigningMethodES256),
		sign("http://issuer", "hydrosmart-api", time.Now().Add(-time.Minute), jwt.SigningMethodES256),
		sign("http://issuer", "hydrosmart-api", time.Now().Add(time.Minute), jwt.SigningMethodHS256),
	} {
		if verifier.Verify(context.Background(), raw) == nil {
			t.Fatal("invalid token accepted")
		}
	}
}
