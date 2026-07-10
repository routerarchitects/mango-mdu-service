package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/config"
	"github.com/routerarchitects/mango-mdu-service/internal/db"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	apphttp "github.com/routerarchitects/mango-mdu-service/internal/http"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/logger"
)

// App encapsulates all application level runtime wiring.
type App struct {
	cfg       *config.Config
	logger    *slog.Logger
	db        *db.Database
	discovery *servicediscovery.Discovery
	httpMod   *apphttp.Module
}

// New initializes all dependencies and builds the App.
func New(ctx context.Context, cfg *config.Config, rootLog *slog.Logger) (*App, error) {
	// Validate discovery configuration dependency
	if !cfg.Discovery.Enabled {
		rootLog.Warn("service discovery is disabled via configuration; business API handlers will not be registered")
	}

	// Validate authentication configuration dependencies
	if cfg.Auth.Enabled {
		if !cfg.Discovery.Enabled || !cfg.RPC.Enabled {
			rootLog.Warn("public authentication (AUTH_ENABLED) is enabled but missing discovery or RPC; token validation may fail")
		}
	}

	// 1. Establish database connection pool
	database, err := db.Connect(ctx, cfg.Database, logger.Subsystem("db"))
	if err != nil {
		return nil, fmt.Errorf("database connection failure: %w", err)
	}

	// 2. Run automated SQL migrations
	if err := database.RunMigrations(ctx, "db/schema"); err != nil {
		database.Close()
		return nil, fmt.Errorf("database schema migration failure: %w", err)
	}

	// 3. Initialize Service Discovery using common mods (conditional)
	var discovery *servicediscovery.Discovery
	if cfg.Discovery.Enabled {
		discovery, err = servicediscovery.New(
			cfg.Discovery.Config,
			cfg.Kafka.Config,
			logger.Subsystem("service-discovery"),
		)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create service discovery instance: %w", err)
		}
	} else {
		rootLog.Info("service discovery is disabled via configuration")
	}

	// 4. Initialize token validator (conditional)
	var tokenValidator auth.PublicAuthValidator
	if !cfg.RPC.Enabled {
		rootLog.Info("service RPC and token validation are disabled via configuration")
	}

	var sessionHandler *handlers.SessionHandler
	var hierarchyHandler *handlers.HierarchyHandler
	var entityHandler *handlers.EntityHandler
	var venueHandler *handlers.VenueHandler
	var assignmentHandler *handlers.AssignmentHandler

	if cfg.Discovery.Enabled {
		provClient, err := prov.NewClient(
			discovery,
			cfg.Server.TLS_ROOTCA,
			cfg.Discovery.PublicEndpoint,
		)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create prov client: %w", err)
		}

		secClient, err := sec.NewClient(
			discovery,
			cfg.Server.TLS_ROOTCA,
			cfg.Discovery.PublicEndpoint,
		)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create sec client: %w", err)
		}
		secClient.AuthEnabled = cfg.Auth.Enabled

		if cfg.RPC.Enabled {
			tokenValidator = sec.NewClientAdapter(secClient)
		}

		sessionService := services.NewSessionService(secClient, provClient)
		sessionHandler = handlers.NewSessionHandler(sessionService, cfg.Auth.Enabled)

		hierarchyService := services.NewHierarchyService(provClient)
		hierarchyHandler = handlers.NewHierarchyHandler(hierarchyService)

		entityService := services.NewEntityService(provClient)
		entityHandler = handlers.NewEntityHandler(entityService)

		venueService := services.NewVenueService(provClient)
		venueHandler = handlers.NewVenueHandler(venueService)

		assignmentService := services.NewAssignmentService(provClient)
		assignmentHandler = handlers.NewAssignmentHandler(assignmentService)
	}

	// 5. Assemble Fiber HTTP apps module
	publicAuthConfig := auth.PublicAuthConfig{}

	expectedAPIKey := cfg.Discovery.InstanceKey
	if cfg.Discovery.Enabled && discovery != nil {
		expectedAPIKey = discovery.Self().Key
	}

	privateAuthConfig := auth.InternalAPIKeyConfig{
		ExpectedAPIKey: expectedAPIKey,
	}

	module, err := apphttp.NewModule(apphttp.Dependencies{
		ServerLogger:      logger.Subsystem("server"),
		ServerConfig:      cfg.Server,
		SubsystemConfig:   cfg.Subsystem.Config,
		PublicAuthConfig:  publicAuthConfig,
		PrivateAuthConfig: privateAuthConfig,
		TokenValidator:    tokenValidator,
		AuthEnabled:       cfg.Auth.Enabled,
		SessionHandler:    sessionHandler,
		HierarchyHandler:  hierarchyHandler,
		EntityHandler:     entityHandler,
		VenueHandler:      venueHandler,
		AssignmentHandler: assignmentHandler,
	})
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to create HTTP module: %w", err)
	}

	return &App{
		cfg:       cfg,
		logger:    rootLog,
		db:        database,
		discovery: discovery,
		httpMod:   module,
	}, nil
}

// Start launches background services and HTTP listeners.
func (a *App) Start(ctx context.Context) (<-chan error, error) {
	// Start service discovery heartbeat loop (conditional)
	if a.cfg.Discovery.Enabled && a.discovery != nil {
		if err := a.discovery.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start service discovery publisher: %w", err)
		}
	}

	// Bind HTTP ports and start Fiber apps
	serverErrCh, err := a.httpMod.Start(ctx)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if a.cfg.Discovery.Enabled && a.discovery != nil {
			_ = a.discovery.Stop(shutdownCtx)
		}
		return nil, fmt.Errorf("failed to start HTTPS listeners: %w", err)
	}

	return serverErrCh, nil
}

// Close performs a graceful shutdown of all resources.
func (a *App) Close(ctx context.Context) error {
	var firstErr error

	if err := a.httpMod.Shutdown(); err != nil {
		a.logger.Error("forced HTTP shutdown occurred", "error", err)
		firstErr = err
	}

	if a.cfg.Discovery.Enabled && a.discovery != nil {
		if err := a.discovery.Stop(ctx); err != nil {
			a.logger.Error("failed to gracefully stop service discovery publisher", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if a.db != nil {
		a.db.Close()
	}

	return firstErr
}
