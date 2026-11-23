package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

const (
	DefaultInternalNameHeader = "X-INTERNAL-NAME"
	DefaultAPIKeyHeader       = "X-API-KEY"
)

var (
	ErrMissingSecretOrKey = errors.New("auth: ConfiguredSecret or DiscoveryKey is required")
)

// CanonicalEndpoint formats an endpoint URI canonically for deterministic SHA-256 derivation.
// Trailing slashes are trimmed, and URL scheme and host are normalized to lower case.
func CanonicalEndpoint(endpoint string) string {
	s := strings.TrimSpace(endpoint)
	if s == "" {
		return ""
	}

	u, err := url.Parse(s)
	if err == nil && u.Host != "" {
		scheme := strings.ToLower(u.Scheme)
		host := strings.ToLower(u.Host)
		path := strings.TrimRight(u.Path, "/")
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if scheme != "" {
			return fmt.Sprintf("%s://%s%s", scheme, host, path)
		}
		return fmt.Sprintf("%s%s", host, path)
	}

	return strings.TrimRight(s, "/")
}

// DeriveKey derives the SHA-256 hex key for service discovery and authentication:
// discoveryKey = SHA256(canonical(publicEndPoint) + ":" + configuredAPISecret)
func DeriveKey(publicEndpoint, secret string) string {
	canonical := CanonicalEndpoint(publicEndpoint)
	secret = strings.TrimSpace(secret)
	if canonical == "" || secret == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(canonical + ":" + secret))
	return hex.EncodeToString(sum[:])
}

// VerifyAPIKey verifies an incoming API key using constant-time comparison against the derived discovery key.
// It supports both incoming raw secrets (incomingKey = SHA256(endpoint + ":" + gotKey)) and direct derived keys.
func VerifyAPIKey(gotKey, publicEndpoint, configuredSecret, expectedDiscoveryKey string) bool {
	gotKey = strings.TrimSpace(gotKey)
	if gotKey == "" {
		return false
	}

	if expectedDiscoveryKey == "" {
		expectedDiscoveryKey = DeriveKey(publicEndpoint, configuredSecret)
	}
	if expectedDiscoveryKey == "" {
		return false
	}

	incomingKey := DeriveKey(publicEndpoint, gotKey)
	matchDerived := subtle.ConstantTimeCompare([]byte(incomingKey), []byte(expectedDiscoveryKey)) == 1
	matchDirect := subtle.ConstantTimeCompare([]byte(gotKey), []byte(expectedDiscoveryKey)) == 1

	return matchDerived || matchDirect
}

// DerivedInternalAPIKeyConfig holds configuration for derived internal service authentication.
type DerivedInternalAPIKeyConfig struct {
	InternalNameHeader string
	APIKeyHeader       string
	PublicEndpoint     string
	ConfiguredSecret   string
	DiscoveryKey       string
	OnUnauthorized     func(c fiber.Ctx) error
}

// RequireDerivedInternalAPIKey enforces internal authentication using caller identity (X-INTERNAL-NAME)
// and constant-time verified SHA-256 derived API key verification.
func RequireDerivedInternalAPIKey(cfg DerivedInternalAPIKeyConfig) (fiber.Handler, error) {
	internalHeader := strings.TrimSpace(cfg.InternalNameHeader)
	if internalHeader == "" {
		internalHeader = DefaultInternalNameHeader
	}

	apiKeyHeader := strings.TrimSpace(cfg.APIKeyHeader)
	if apiKeyHeader == "" {
		apiKeyHeader = DefaultAPIKeyHeader
	}

	discoveryKey := cfg.DiscoveryKey
	if discoveryKey == "" && cfg.ConfiguredSecret != "" && cfg.PublicEndpoint != "" {
		discoveryKey = DeriveKey(cfg.PublicEndpoint, cfg.ConfiguredSecret)
	}

	if discoveryKey == "" && cfg.ConfiguredSecret == "" {
		return nil, ErrMissingSecretOrKey
	}

	unauthorized := cfg.OnUnauthorized
	if unauthorized == nil {
		unauthorized = func(c fiber.Ctx) error {
			return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
				http.StatusUnauthorized, "Unauthorized",
			))
		}
	}

	return func(c fiber.Ctx) error {
		internalName := strings.TrimSpace(c.Get(internalHeader))
		if internalName == "" {
			return unauthorized(c)
		}

		gotKey := strings.TrimSpace(c.Get(apiKeyHeader))
		if gotKey == "" {
			return unauthorized(c)
		}

		if !VerifyAPIKey(gotKey, cfg.PublicEndpoint, cfg.ConfiguredSecret, discoveryKey) {
			return unauthorized(c)
		}

		c.Locals("internal_name", internalName)
		return c.Next()
	}, nil
}
