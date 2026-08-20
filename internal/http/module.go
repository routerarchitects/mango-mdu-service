package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/config"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/http/routes"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	subsystemroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type Dependencies struct {
	ServerLogger      *slog.Logger
	ServerConfig      config.ServerConfig
	SubsystemConfig   subsystemroutes.Config
	PublicAuthConfig  auth.PublicAuthConfig
	PrivateAuthConfig auth.InternalAPIKeyConfig
	TokenValidator    auth.PublicAuthValidator
	AuthEnabled       bool
	DashboardHandler  *handlers.DashboardHandler
}

type Module struct {
	server     *Server
	publicApp  *fiber.App
	privateApp *fiber.App
}

func NewModule(deps Dependencies) (*Module, error) {
	if deps.DashboardHandler == nil {
		if deps.ServerLogger != nil {
			deps.ServerLogger.Warn("dashboard handler is nil; dashboard API endpoint will not be registered")
		}
	}

	authMiddleware, err := middleware.NewServiceAuth(
		deps.AuthEnabled,
		deps.PublicAuthConfig,
		deps.PrivateAuthConfig,
		deps.TokenValidator,
	)
	if err != nil {
		return nil, err
	}

	appConfig := fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	}

	publicApp := fiber.New(appConfig)
	privateApp := fiber.New(appConfig)

	// Register CORS policy for external UI calls
	middleware.RegisterPublicCORS(publicApp)

	// Register request/correlation ID normalization middleware
	publicApp.Use(middleware.CorrelationAndRequestID())
	privateApp.Use(middleware.CorrelationAndRequestID())

	// Register trace loggers
	middleware.RegisterRequestLog(publicApp, deps.ServerLogger)
	middleware.RegisterRequestLog(privateApp, deps.ServerLogger)

	// Configure public routes
	routes.RegisterPublic(publicApp, routes.PublicDeps{
		AuthHandler:      authMiddleware.GetPublicAuthHandler(),
		Subsystem:        deps.SubsystemConfig,
		DashboardHandler: deps.DashboardHandler,
	})

	// Configure private routes
	routes.RegisterPrivate(privateApp, routes.PrivateDeps{
		AuthHandler: authMiddleware.GetPrivateAuthHandler(),
		Subsystem:   deps.SubsystemConfig,
	})

	return &Module{
		server:     NewServer(deps.ServerConfig, deps.ServerLogger),
		publicApp:  publicApp,
		privateApp: privateApp,
	}, nil
}

// Start launches the public and private HTTP listener servers in the background.
func (m *Module) Start(ctx context.Context) (<-chan error, error) {
	return m.server.Start(ctx, m.publicApp, m.privateApp)
}

// Shutdown gracefully stops both Fiber listeners.
func (m *Module) Shutdown() error {
	var errs []error
	if err := m.publicApp.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("public application shutdown: %w", err))
	}
	if err := m.privateApp.Shutdown(); err != nil {
		errs = append(errs, fmt.Errorf("private application shutdown: %w", err))
	}
	return errors.Join(errs...)
}
