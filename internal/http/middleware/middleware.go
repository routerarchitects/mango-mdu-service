package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/requestlog"
)

// RegisterPublicCORS configures CORS policies on the public Fiber application.
func RegisterPublicCORS(app *fiber.App) {
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-KEY", "X-INTERNAL-NAME"},
	}))
}

// CorrelationAndRequestID normalizes, validates, and generates request/correlation IDs.
func CorrelationAndRequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Read incoming request ID
		reqID := strings.TrimSpace(c.Get("X-Request-Id"))
		if reqID == "" {
			reqID = strings.TrimSpace(c.Get("X-Request-ID"))
		}

		if !isValidID(reqID) {
			// Generate a new one if missing or invalid using the common-mods algorithm
			var b [12]byte
			if _, err := rand.Read(b[:]); err != nil {
				reqID = fmt.Sprintf("%d", time.Now().UnixNano())
			} else {
				reqID = hex.EncodeToString(b[:])
			}
		}

		// Read incoming correlation ID
		corrID := strings.TrimSpace(c.Get("X-Correlation-Id"))
		if corrID == "" {
			corrID = strings.TrimSpace(c.Get("X-Correlation-ID"))
		}

		if corrID == "" {
			// Fall back to request ID if correlation ID is not supplied
			corrID = reqID
		} else if !isValidID(corrID) {
			// Discard invalid correlation ID and fall back to request ID
			corrID = reqID
		}

		// Standardize request headers for downstream context
		c.Request().Header.Set("X-Request-ID", reqID)
		c.Request().Header.Set("X-Request-Id", reqID)

		c.Request().Header.Set("X-Correlation-ID", corrID)
		c.Request().Header.Set("X-Correlation-Id", corrID)

		// Standardize response headers
		c.Response().Header.Set("X-Request-ID", reqID)
		c.Response().Header.Set("X-Request-Id", reqID)
		c.Response().Header.Set("X-Correlation-ID", corrID)
		c.Response().Header.Set("X-Correlation-Id", corrID)

		return c.Next()
	}
}

func isValidID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == ':' || ch == '.') {
			return false
		}
	}
	return true
}

// RegisterRequestLog registers the correlation and structured request logger middleware.
func RegisterRequestLog(app *fiber.App, logger *slog.Logger) {
	app.Use(requestlog.RequestLogger(logger))
}

// ServiceAuth manages public and private authentication middleware state.
type ServiceAuth struct {
	publicAuth  fiber.Handler
	privateAuth fiber.Handler
}

// NewServiceAuth creates and configures public and private auth handlers.
func NewServiceAuth(
	authEnabled bool,
	publicCfg auth.PublicAuthConfig,
	privateCfg auth.InternalAPIKeyConfig,
	validator auth.PublicAuthValidator,
) (*ServiceAuth, error) {
	// Configure public auth handler (bypassed if AUTH_ENABLED=false)
	var publicAuth fiber.Handler
	if !authEnabled {
		publicAuth = func(c fiber.Ctx) error {
			return c.Next()
		}
	} else {
		if publicCfg.Validator == nil {
			publicCfg.Validator = validator
		}
		var err error
		publicAuth, err = auth.RequirePublicAuth(publicCfg)
		if err != nil {
			return nil, err
		}
	}

	// Configure private auth handler (always enforced for security)
	privateAuth, err := auth.RequireInternalAPIKey(privateCfg)
	if err != nil {
		return nil, err
	}

	return &ServiceAuth{
		publicAuth:  publicAuth,
		privateAuth: privateAuth,
	}, nil
}

// GetPublicAuthHandler returns the public/bearer authentication middleware.
func (sa *ServiceAuth) GetPublicAuthHandler() fiber.Handler {
	return sa.publicAuth
}

// GetPrivateAuthHandler returns the private/internal API key authentication middleware.
func (sa *ServiceAuth) GetPrivateAuthHandler() fiber.Handler {
	return sa.privateAuth
}
