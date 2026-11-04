package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type JwtTokenService struct {
	URL, Issuer, Audience string
	Client                *http.Client
	mu                    sync.Mutex
	keys                  map[string]*ecdsa.PublicKey
	fetched               time.Time
}

func (s *JwtTokenService) key(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.keys[kid]
	if time.Since(s.fetched) < 5*time.Minute && key != nil {
		return key, nil
	}
	if time.Since(s.fetched) < 5*time.Second && key == nil {
		return nil, valueobjects.ErrUnauthorized
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
	res, err := s.Client.Do(req)
	if err != nil {
		return nil, valueobjects.ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, valueobjects.ErrUnavailable
	}
	var jwks struct {
		Keys []struct{ Kty, Crv, Alg, Kid, X, Y string }
	}
	if json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&jwks) != nil {
		return nil, valueobjects.ErrUnavailable
	}
	keys := map[string]*ecdsa.PublicKey{}
	for _, k := range jwks.Keys {
		if k.Kty != "EC" || k.Crv != "P-256" || (k.Alg != "" && k.Alg != "ES256") || k.Kid == "" {
			continue
		}
		x, e1 := base64.RawURLEncoding.DecodeString(k.X)
		y, e2 := base64.RawURLEncoding.DecodeString(k.Y)
		if e1 != nil || e2 != nil || len(x) != 32 || len(y) != 32 {
			continue
		}
		pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if pk.Curve.IsOnCurve(pk.X, pk.Y) {
			keys[k.Kid] = pk
		}
	}
	s.keys = keys
	s.fetched = time.Now()
	key = keys[kid]
	if key == nil {
		return nil, valueobjects.ErrUnauthorized
	}
	return key, nil
}
func (s *JwtTokenService) Verify(ctx context.Context, raw string) error {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, valueobjects.ErrUnauthorized
		}
		return s.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(s.Issuer), jwt.WithAudience(s.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(5*time.Second))
	if err != nil {
		return err
	}
	if !token.Valid {
		return valueobjects.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return valueobjects.ErrUnauthorized
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return valueobjects.ErrUnauthorized
	}
	profile, ok := claims["profile_type"].(string)
	if !ok || (profile != "propietario" && profile != "inquilino") {
		return valueobjects.ErrUnauthorized
	}
	address, parseErr := url.Parse(s.URL)
	if parseErr != nil {
		return valueobjects.ErrUnavailable
	}
	address.Path = "/internal/v1/users/" + url.PathEscape(sub)
	address.RawQuery = ""
	req, _ := http.NewRequestWithContext(ctx, "GET", address.String(), nil)
	req.Header.Set("X-Service-Token", os.Getenv("INTERNAL_SERVICE_TOKEN"))
	res, requestErr := s.Client.Do(req)
	if requestErr != nil {
		return valueobjects.ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return valueobjects.ErrUnauthorized
	}
	if res.StatusCode != 200 {
		return valueobjects.ErrUnavailable
	}
	return nil
}
