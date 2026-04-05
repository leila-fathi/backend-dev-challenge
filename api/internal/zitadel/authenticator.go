package zitadel

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidAccessToken = errors.New("invalid access token")
)

const (
	discoveryPath = "/.well-known/openid-configuration"

	defaultHTTPTimeout  = 10 * time.Second
	defaultJWKSMaxAge   = 10 * time.Minute
	maxResponseBodySize = 1 << 20 // 1 MiB
	maxErrorBodyChars   = 400
)

type Verifier interface {
	VerifyAccessToken(ctx context.Context, rawToken string) (*Identity, error)
}

type Identity struct {
	Subject string
	Email   string
}

type openIDConfigResponse struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type jwksResponse struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	KeyType string `json:"kty"`
	KeyID   string `json:"kid"`
	Use     string `json:"use"`
	N       string `json:"n"`
	E       string `json:"e"`
}

type Authenticator struct {
	baseURL    string
	httpClient *http.Client

	mu sync.Mutex

	discoveredIssuer string
	jwksKeys         map[string]*rsa.PublicKey
	jwksFetchedAt    time.Time
}

func NewAuthenticator(domain, browserClientID, serviceAccountKeyPath string) (*Authenticator, error) {
	_ = browserClientID
	_ = serviceAccountKeyPath

	baseURL := strings.TrimSpace(domain)
	if baseURL == "" {
		return nil, errors.New("zitadel domain is required")
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Authenticator{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		discoveredIssuer: baseURL,
		jwksKeys:         make(map[string]*rsa.PublicKey),
	}, nil
}

func (a *Authenticator) VerifyAccessToken(ctx context.Context, rawToken string) (*Identity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidAccessToken
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, a.keyFunc(ctx), jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))
	if err != nil {
		return nil, fmt.Errorf("verify access token: %w", ErrInvalidAccessToken)
	}
	if token == nil || !token.Valid {
		return nil, fmt.Errorf("verify access token: %w", ErrInvalidAccessToken)
	}

	issuerRaw, _ := claims["iss"].(string)
	if !a.matchesIssuer(issuerRaw) {
		return nil, fmt.Errorf("verify access token: %w", ErrInvalidAccessToken)
	}

	subject, _ := claims["sub"].(string)
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, fmt.Errorf("verify access token: %w", ErrInvalidAccessToken)
	}

	email := strings.TrimSpace(firstNonEmpty(
		claimAsString(claims, "email"),
		claimAsString(claims, "preferred_username"),
		claimAsString(claims, "username"),
	))

	identity := &Identity{
		Subject: subject,
		Email:   email,
	}

	return identity, nil
}

func (a *Authenticator) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, fmt.Errorf("missing kid header: %w", ErrInvalidAccessToken)
		}

		key, err := a.getJWKByKeyID(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
}

func (a *Authenticator) matchesIssuer(issuer string) bool {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return false
	}
	expected := strings.TrimSpace(a.currentIssuer())
	if expected == "" {
		return false
	}
	return strings.TrimRight(issuer, "/") == strings.TrimRight(expected, "/")
}

func (a *Authenticator) currentIssuer() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(a.discoveredIssuer) != "" {
		return a.discoveredIssuer
	}
	return a.baseURL
}

func (a *Authenticator) getJWKByKeyID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, err := a.getJWKCache(ctx, false)
	if err != nil {
		return nil, err
	}
	if key, ok := keys[kid]; ok {
		return key, nil
	}

	// Refresh once in case of key rotation.
	keys, err = a.getJWKCache(ctx, true)
	if err != nil {
		return nil, err
	}
	if key, ok := keys[kid]; ok {
		return key, nil
	}

	return nil, fmt.Errorf("unknown key id %q: %w", kid, ErrInvalidAccessToken)
}

func (a *Authenticator) getJWKCache(ctx context.Context, forceRefresh bool) (map[string]*rsa.PublicKey, error) {
	if !forceRefresh {
		a.mu.Lock()
		if len(a.jwksKeys) > 0 && time.Since(a.jwksFetchedAt) < defaultJWKSMaxAge {
			cached := cloneJWKMap(a.jwksKeys)
			a.mu.Unlock()
			return cached, nil
		}
		a.mu.Unlock()
	}

	cfg, err := a.fetchOpenIDConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	keys, err := a.fetchJWKS(ctx, cfg.JWKSURI)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.discoveredIssuer = strings.TrimSpace(cfg.Issuer)
	a.jwksKeys = cloneJWKMap(keys)
	a.jwksFetchedAt = time.Now().UTC()
	cached := cloneJWKMap(a.jwksKeys)
	a.mu.Unlock()

	return cached, nil
}

func (a *Authenticator) fetchOpenIDConfiguration(ctx context.Context) (*openIDConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+discoveryPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build openid configuration request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute openid configuration request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, fmt.Errorf("read openid configuration response body: %w", err)
	}
	if !is2xx(resp.StatusCode) {
		return nil, statusError("fetch openid configuration", resp.StatusCode, raw)
	}

	var cfg openIDConfigResponse
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode openid configuration response: %w", err)
	}

	cfg.Issuer = strings.TrimSpace(cfg.Issuer)
	cfg.JWKSURI = strings.TrimSpace(cfg.JWKSURI)
	if cfg.Issuer == "" {
		cfg.Issuer = a.baseURL
	}
	if cfg.JWKSURI == "" {
		return nil, errors.New("openid configuration missing jwks_uri")
	}
	return &cfg, nil
}

func (a *Authenticator) fetchJWKS(ctx context.Context, jwksURI string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute jwks request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, fmt.Errorf("read jwks response body: %w", err)
	}
	if !is2xx(resp.StatusCode) {
		return nil, statusError("fetch jwks", resp.StatusCode, raw)
	}

	var payload jwksResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode jwks response: %w", err)
	}
	if len(payload.Keys) == 0 {
		return nil, errors.New("jwks response has no keys")
	}

	out := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, key := range payload.Keys {
		if strings.TrimSpace(key.KeyType) != "RSA" {
			continue
		}
		kid := strings.TrimSpace(key.KeyID)
		if kid == "" {
			continue
		}
		pub, err := parseRSAPublicKeyFromJWK(key.N, key.E)
		if err != nil {
			return nil, fmt.Errorf("parse jwk %q: %w", kid, err)
		}
		out[kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("jwks has no usable rsa keys")
	}
	return out, nil
}

func parseRSAPublicKeyFromJWK(nBase64URL, eBase64URL string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nBase64URL)
	if err != nil {
		return nil, fmt.Errorf("decode modulus n: %w", err)
	}
	eBytes, err := decodeBase64URL(eBase64URL)
	if err != nil {
		return nil, fmt.Errorf("decode exponent e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if n.Sign() <= 0 || e.Sign() <= 0 {
		return nil, errors.New("invalid rsa key parameters")
	}
	if !e.IsInt64() {
		return nil, errors.New("rsa exponent out of range")
	}
	eInt := int(e.Int64())
	if eInt <= 0 {
		return nil, errors.New("invalid rsa exponent")
	}

	return &rsa.PublicKey{N: n, E: eInt}, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty base64url value")
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	// Fallback to standard Base64URL decoding.
	return base64.URLEncoding.DecodeString(value)
}

func cloneJWKMap(src map[string]*rsa.PublicKey) map[string]*rsa.PublicKey {
	out := make(map[string]*rsa.PublicKey, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func is2xx(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func statusError(op string, status int, body []byte) error {
	return fmt.Errorf("%s failed: %s", op, statusSummary(status, body))
}

func statusSummary(status int, body []byte) string {
	return fmt.Sprintf("status=%d (%s) body=%s", status, http.StatusText(status), truncateBody(body))
}

func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxErrorBodyChars {
		return s[:maxErrorBodyChars] + "..."
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func claimAsString(claims jwt.MapClaims, key string) string {
	raw, ok := claims[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}
