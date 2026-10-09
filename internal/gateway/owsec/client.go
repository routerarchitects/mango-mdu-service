package owsec

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const (
	userAgent     = "mango-mdu-service/1.0"
	pageSize      = 500
	maxErrorBytes = 2048
)

// Client defines the contract for communicating with OWSEC.
type Client interface {
	GetUsers(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error)
}

// InstanceResolver resolves an instance's endpoint and key together from a single discovered instance.
type InstanceResolver func() (endpoint string, key string, err error)

type client struct {
	instanceResolver InstanceResolver
	urlResolver      func() string
	keyResolver      func() string
	internalName     string
	internalKey      string
	httpClient       *http.Client
	logger           *slog.Logger
}

// Config holds configuration for creating an OWSEC client.
type Config struct {
	InstanceResolver InstanceResolver
	URLResolver      func() string
	KeyResolver      func() string
	InternalName     string
	InternalKey      string
	Timeout          time.Duration
	TLSConfig        *tls.Config
	Logger           *slog.Logger
}

// NewClient creates a new OWSEC client instance.
func NewClient(cfg Config) Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.TLSConfig != nil {
		transport.TLSClientConfig = cfg.TLSConfig
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	internalName := strings.TrimSpace(cfg.InternalName)
	if internalName == "" {
		internalName = "mango-mdu-service"
	}

	return &client{
		instanceResolver: cfg.InstanceResolver,
		urlResolver:      cfg.URLResolver,
		keyResolver:      cfg.KeyResolver,
		internalName:     internalName,
		internalKey:      cfg.InternalKey,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		logger: logger,
	}
}

func (c *client) resolveTarget() (string, string, error) {
	if c.instanceResolver != nil {
		ep, key, err := c.instanceResolver()
		if err != nil {
			var apiErr models.ApiError
			if errors.As(err, &apiErr) {
				return "", "", err
			}
			return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owsec service endpoint not discovered or available")
		}
		if ep == "" {
			return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owsec service endpoint not discovered or available")
		}
		if strings.TrimSpace(key) == "" {
			return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owsec service API key not discovered or available")
		}
		return strings.TrimRight(ep, "/"), key, nil
	}

	var ep string
	if c.urlResolver != nil {
		ep = c.urlResolver()
	}
	if ep == "" {
		return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owsec service endpoint not discovered or available")
	}

	var key string
	if c.keyResolver != nil {
		key = c.keyResolver()
	} else {
		key = c.internalKey
	}
	if strings.TrimSpace(key) == "" {
		return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owsec service API key not discovered or available")
	}
	return strings.TrimRight(ep, "/"), key, nil
}

// GetUsers retrieves all accessible users from OWSEC using pagination.
func (c *client) GetUsers(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error) {
	baseURL, apiKey, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	var allUsers []models.SecUser
	offset := 0

	for {
		endpoint := fmt.Sprintf("%s/api/v1/users?limit=%d&offset=%d", baseURL, pageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create users request", err)
		}

		c.setHeaders(req, apiKey, token, reqID, corrID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("downstream OWSEC request failed",
					"service", "owsec",
					"error", err,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWSEC service unreachable")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
			_ = body
			resp.Body.Close()
			if c.logger != nil {
				c.logger.Error("downstream OWSEC returned non-200 status",
					"service", "owsec",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(resp.StatusCode, "Downstream Error", fmt.Sprintf("downstream OWSEC returned status %d", resp.StatusCode))
		}

		var usersResp models.SecUserListResponse
		err = json.NewDecoder(resp.Body).Decode(&usersResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse users response", err)
		}

		allUsers = append(allUsers, usersResp.Users...)

		if len(usersResp.Users) < pageSize {
			break
		}
		offset += pageSize
	}

	return allUsers, nil
}

func (c *client) setHeaders(req *http.Request, key, token, reqID, corrID string) {
	req.Header.Set("X-INTERNAL-NAME", c.internalName)
	req.Header.Set("X-API-KEY", key)
	if token != "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.Header.Set("Authorization", token)
		}
	}
	req.Header.Set("User-Agent", userAgent)
	if reqID != "" {
		req.Header.Set("X-Request-Id", reqID)
	}
	if corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}
}
