package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/config"
	"github.com/routerarchitects/mango-mdu-service/internal/db"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owsec"
	apphttp "github.com/routerarchitects/mango-mdu-service/internal/http"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ow-common-mods/servicerpc"
	owsecrpc "github.com/routerarchitects/ow-common-mods/servicerpc/owsec"
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
	// Validate authentication configuration dependencies
	if cfg.Auth.Enabled {
		if !cfg.Discovery.Enabled || !cfg.RPC.Enabled {
			return nil, fmt.Errorf("invalid configuration: public authentication (AUTH_ENABLED) requires both service discovery (DISCOVERY_ENABLED) and service RPC (SERVICE_RPC_ENABLED) to be enabled")
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

	// 4. Initialize RPC client factory (conditional)
	var tokenValidator *owsecrpc.SecurityClient
	if cfg.RPC.Enabled && cfg.Discovery.Enabled {
		rpcFactory, err := servicerpc.NewServiceRpc(
			discovery,
			servicerpc.ServiceRpcConfig{
				TLSRootCA:    cfg.Server.TLS_ROOTCA,
				InternalName: cfg.Discovery.PublicEndpoint,
			},
			logger.Subsystem("service-rpc"),
		)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create service RPC factory: %w", err)
		}

		// Retrieve Security Client validator
		tokenValidator, err = rpcFactory.SecurityClient()
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create security auth client: %w", err)
		}
	} else {
		rootLog.Info("service RPC client factory and token validation are disabled via configuration")
	}

	// 5. Initialize TLS configuration for downstream microservices
	var tlsConfig *tls.Config
	caPath := strings.TrimSpace(cfg.Server.TLS_ROOTCA)
	if caPath != "" {
		pemBytes, err := os.ReadFile(caPath)
		if err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to read TLS root CA %q: %w", caPath, err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			database.Close()
			return nil, fmt.Errorf("failed to parse TLS root CA %q: invalid PEM format", caPath)
		}
		tlsConfig = &tls.Config{RootCAs: pool}
	}

	// 6. Initialize downstream gateway clients with dynamic discovery
	owprovClient := owprov.NewClient(owprov.Config{
		URLResolver: func() string {
			if discovery != nil {
				if inst := discovery.Store().GetServiceInstances("owprov"); inst != nil {
					return inst.PrivateEndPoint
				}
			}
			return ""
		},
		TLSConfig: tlsConfig,
		Logger:    rootLog,
	})

	instanceKey := cfg.Discovery.InstanceKey
	if instanceKey == "" && discovery != nil {
		instanceKey = discovery.Self().Key
	}
	if strings.TrimSpace(instanceKey) == "" && cfg.Auth.Enabled {
		rootLog.Warn("internal API key is empty; set discovery.instance_key or enable discovery for authenticated internal requests")
	}

	instanceInternalName := ""
	if discovery != nil {
		instanceInternalName = discovery.Self().PrivateEndPoint
	}
	if instanceInternalName == "" {
		instanceInternalName = strings.TrimSpace(cfg.Discovery.PublicEndpoint)
	}
	if instanceInternalName == "" {
		instanceInternalName = "mango-mdu-service"
	}

	owsecClient := owsec.NewClient(owsec.Config{
		InstanceResolver: func() (string, string, error) {
			if discovery != nil {
				if inst := discovery.Store().GetServiceInstances("owsec"); inst != nil {
					return inst.PrivateEndPoint, inst.Key, nil
				}
			}
			return "", "", fmt.Errorf("owsec service endpoint not discovered or available")
		},
		InternalName: instanceInternalName,
		TLSConfig:    tlsConfig,
		Logger:       rootLog,
	})

	policyService := services.NewPolicyService(owprovClient, owsecClient)
	policyHandler := handlers.NewPolicyHandler(policyService, rootLog)

	// 7. Assemble Fiber HTTP apps module
	publicAuthConfig := auth.PublicAuthConfig{}
	privateAuthConfig := auth.InternalAPIKeyConfig{
		ExpectedAPIKey: instanceKey,
	}

	module, err := apphttp.NewModule(apphttp.Dependencies{
		ServerLogger:      logger.Subsystem("server"),
		ServerConfig:      cfg.Server,
		SubsystemConfig:   cfg.Subsystem.Config,
		PublicAuthConfig:  publicAuthConfig,
		PrivateAuthConfig: privateAuthConfig,
		TokenValidator:    tokenValidator,
		AuthEnabled:       cfg.Auth.Enabled,
		PolicyHandler:     policyHandler,
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
