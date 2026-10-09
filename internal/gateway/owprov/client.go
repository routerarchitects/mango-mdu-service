package owprov

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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

// Client defines the contract for communicating with OWPROV.
type Client interface {
	GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error)
	GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error)
	GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error)
	GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error)
}

type client struct {
	urlResolver func() string
	httpClient  *http.Client
	logger      *slog.Logger
}

// Config holds configuration for creating an OWPROV client.
type Config struct {
	URLResolver func() string
	Timeout     time.Duration
	TLSConfig   *tls.Config
	Logger      *slog.Logger
}

// NewClient creates a new OWPROV client instance.
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

	return &client{
		urlResolver: cfg.URLResolver,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		logger: logger,
	}
}

func (c *client) getBaseURL() (string, error) {
	if c.urlResolver != nil {
		if resolved := c.urlResolver(); resolved != "" {
			return strings.TrimRight(resolved, "/"), nil
		}
	}
	return "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov service endpoint not discovered or available")
}

// GetPolicy retrieves a single management policy by ID from OWPROV.
func (c *client) GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
	baseURL, err := c.getBaseURL()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/v1/managementPolicy/%s", baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create policy request", err)
	}

	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV request failed",
				"service", "owprov",
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, models.NewApiError(http.StatusNotFound, "Not Found", "Management policy not found")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		_ = body
		if c.logger != nil {
			c.logger.Error("downstream OWPROV returned non-200 status",
				"service", "owprov",
				"status", resp.StatusCode,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.NewApiError(resp.StatusCode, "Downstream Error", fmt.Sprintf("downstream OWPROV returned status %d", resp.StatusCode))
	}

	var policy models.ManagementPolicy
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse policy response", err)
	}

	return &policy, nil
}

// GetRolesByPolicy fetches all management roles associated with policyID using pagination.
func (c *client) GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
	baseURL, err := c.getBaseURL()
	if err != nil {
		return nil, err
	}

	var allRoles []models.ManagementRole
	offset := 0

	for {
		endpoint := fmt.Sprintf("%s/api/v1/managementRole?policyId=%s&limit=%d&offset=%d",
			baseURL, url.QueryEscape(policyID), pageSize, offset)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create roles request", err)
		}

		c.setHeaders(req, token, reqID, corrID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("downstream OWPROV request failed",
					"service", "owprov",
					"error", err,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
			_ = body
			resp.Body.Close()
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(resp.StatusCode, "Downstream Error", fmt.Sprintf("downstream OWPROV returned status %d", resp.StatusCode))
		}

		var roleResp models.ManagementRoleListResponse
		err = json.NewDecoder(resp.Body).Decode(&roleResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse roles response", err)
		}

		allRoles = append(allRoles, roleResp.Roles...)

		if len(roleResp.Roles) < pageSize {
			break
		}
		offset += pageSize
	}

	return allRoles, nil
}

// GetEntities fetches properties/entities from OWPROV using pagination.
func (c *client) GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
	baseURL, err := c.getBaseURL()
	if err != nil {
		return nil, err
	}

	var allEntities []models.Entity
	offset := 0

	for {
		endpoint := fmt.Sprintf("%s/api/v1/entity?limit=%d&offset=%d", baseURL, pageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create entity request", err)
		}

		c.setHeaders(req, token, reqID, corrID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("downstream OWPROV request failed",
					"service", "owprov",
					"error", err,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
			_ = body
			resp.Body.Close()
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(resp.StatusCode, "Downstream Error", fmt.Sprintf("downstream OWPROV returned status %d", resp.StatusCode))
		}

		var entityResp models.EntityListResponse
		err = json.NewDecoder(resp.Body).Decode(&entityResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse entities response", err)
		}

		allEntities = append(allEntities, entityResp.Entities...)

		if len(entityResp.Entities) < pageSize {
			break
		}
		offset += pageSize
	}

	return allEntities, nil
}

// GetVenues fetches venues from OWPROV using pagination.
func (c *client) GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
	baseURL, err := c.getBaseURL()
	if err != nil {
		return nil, err
	}

	var allVenues []models.Venue
	offset := 0

	for {
		endpoint := fmt.Sprintf("%s/api/v1/venue?limit=%d&offset=%d", baseURL, pageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create venue request", err)
		}

		c.setHeaders(req, token, reqID, corrID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("downstream OWPROV request failed",
					"service", "owprov",
					"error", err,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
			_ = body
			resp.Body.Close()
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(resp.StatusCode, "Downstream Error", fmt.Sprintf("downstream OWPROV returned status %d", resp.StatusCode))
		}

		var venueResp models.VenueListResponse
		err = json.NewDecoder(resp.Body).Decode(&venueResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse venues response", err)
		}

		allVenues = append(allVenues, venueResp.Venues...)

		if len(venueResp.Venues) < pageSize {
			break
		}
		offset += pageSize
	}

	return allVenues, nil
}

func (c *client) setHeaders(req *http.Request, token, reqID, corrID string) {
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
