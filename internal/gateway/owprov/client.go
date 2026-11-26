package owprov

import (
	"bytes"
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
	userAgent        = "mango-mdu-service/1.0"
	pageSize         = 500
	maxPages         = 100
	maxErrorBytes    = 2048
	maxResponseBytes = 10 << 20 // 10 MB
)

// InstanceResolver resolves an instance's endpoint and key together from a single discovered instance.
type InstanceResolver func() (endpoint string, key string, err error)

// Client defines the contract for communicating with OWPROV.
type Client interface {
	GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error)
	GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error)
	GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error)
	GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error)
	GetVenue(ctx context.Context, id, token, reqID, corrID string) (*models.VenueDetails, error)
	UpdateVenue(ctx context.Context, id string, body []byte, token, reqID, corrID string) error
	GetInventory(ctx context.Context, token, reqID, corrID string, limit, offset int, unassigned bool) ([]models.InventoryTag, error)
	GetInventoryCount(ctx context.Context, token, reqID, corrID string, unassigned bool) (int, error)
	GetInventorySerials(ctx context.Context, token, reqID, corrID string) ([]string, error)
	GetInventoryItem(ctx context.Context, serial, token, reqID, corrID string) (*models.InventoryTag, error)
	DeleteInventoryItem(ctx context.Context, serial, token, reqID, corrID string) error
	ApplyDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error)
}

type client struct {
	instanceResolver InstanceResolver
	internalName     string
	httpClient       *http.Client
	applyHTTPClient  *http.Client
	logger           *slog.Logger
}

// Config holds configuration for creating an OWPROV client.
type Config struct {
	// InstanceResolver must return the endpoint and key from the same OWPROV instance.
	InstanceResolver InstanceResolver
	InternalName     string
	Timeout          time.Duration
	// ApplyTimeout must cover OWPROV configuration compilation plus its
	// downstream OWGW command timeout.
	ApplyTimeout time.Duration
	TLSConfig    *tls.Config
	Logger       *slog.Logger
}

// NewClient creates a new OWPROV client instance.
func NewClient(cfg Config) *client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	applyTimeout := cfg.ApplyTimeout
	if applyTimeout <= 0 {
		applyTimeout = timeout
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
		instanceResolver: cfg.InstanceResolver,
		internalName:     cfg.InternalName,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		applyHTTPClient: &http.Client{
			Timeout:   applyTimeout,
			Transport: transport,
		},
		logger: logger,
	}
}

func (c *client) resolveTarget() (string, string, error) {
	if c.instanceResolver == nil {
		return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov service endpoint not discovered or available")
	}
	endpoint, key, err := c.instanceResolver()
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if err != nil || endpoint == "" {
		return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov service endpoint not discovered or available")
	}
	if strings.TrimSpace(key) == "" {
		c.logger.Error("OWPROV service key missing from resolved target", "service", "owprov")
		return "", "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov service credentials unavailable")
	}
	return endpoint, key, nil
}

// GetPolicy retrieves a single management policy by ID from OWPROV.
func (c *client) GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/v1/managementPolicy/%s", baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create policy request", err)
	}

	c.setHeaders(req, key, token, reqID, corrID)

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
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	err = json.NewDecoder(limitedReader).Decode(&policy)
	resp.Body.Close()
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse policy response", err)
	}

	return &policy, nil
}

// GetRolesByPolicy fetches all management roles associated with policyID using pagination.
func (c *client) GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	var allRoles []models.ManagementRole
	offset := 0
	pages := 0

	for {
		pages++
		endpoint := fmt.Sprintf("%s/api/v1/managementRole?policyId=%s&limit=%d&offset=%d",
			baseURL, url.QueryEscape(policyID), pageSize, offset)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create roles request", err)
		}

		c.setHeaders(req, key, token, reqID, corrID)

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
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.ParseResponseError(resp, maxErrorBytes)
		}

		var roleResp models.ManagementRoleListResponse
		limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
		err = json.NewDecoder(limitedReader).Decode(&roleResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse roles response", err)
		}

		allRoles = append(allRoles, roleResp.Roles...)

		if len(roleResp.Roles) < pageSize || pages >= maxPages {
			if len(roleResp.Roles) == pageSize && pages >= maxPages && c.logger != nil {
				c.logger.Warn("pagination reached maximum allowed pages", "service", "owprov", "endpoint", "managementRole", "max_pages", maxPages, "total_records", len(allRoles))
			}
			break
		}
		offset += pageSize
	}

	return allRoles, nil
}

// GetEntities fetches properties/entities from OWPROV using pagination.
func (c *client) GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	var allEntities []models.Entity
	offset := 0
	pages := 0

	for {
		pages++
		endpoint := fmt.Sprintf("%s/api/v1/entity?limit=%d&offset=%d", baseURL, pageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create entity request", err)
		}

		c.setHeaders(req, key, token, reqID, corrID)

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
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.ParseResponseError(resp, maxErrorBytes)
		}

		var entityResp models.EntityListResponse
		limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
		err = json.NewDecoder(limitedReader).Decode(&entityResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse entities response", err)
		}

		allEntities = append(allEntities, entityResp.Entities...)

		if len(entityResp.Entities) < pageSize || pages >= maxPages {
			if len(entityResp.Entities) == pageSize && pages >= maxPages && c.logger != nil {
				c.logger.Warn("pagination reached maximum allowed pages", "service", "owprov", "endpoint", "entity", "max_pages", maxPages, "total_records", len(allEntities))
			}
			break
		}
		offset += pageSize
	}

	return allEntities, nil
}

// GetVenues fetches venues from OWPROV using pagination.
func (c *client) GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	var allVenues []models.Venue
	offset := 0
	pages := 0

	for {
		pages++
		endpoint := fmt.Sprintf("%s/api/v1/venue?limit=%d&offset=%d", baseURL, pageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create venue request", err)
		}

		c.setHeaders(req, key, token, reqID, corrID)

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
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.ParseResponseError(resp, maxErrorBytes)
		}

		var venueResp models.VenueListResponse
		limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
		err = json.NewDecoder(limitedReader).Decode(&venueResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse venues response", err)
		}

		allVenues = append(allVenues, venueResp.Venues...)

		if len(venueResp.Venues) < pageSize || pages >= maxPages {
			if len(venueResp.Venues) == pageSize && pages >= maxPages && c.logger != nil {
				c.logger.Warn("pagination reached maximum allowed pages", "service", "owprov", "endpoint", "venue", "max_pages", maxPages, "total_records", len(allVenues))
			}
			break
		}
		offset += pageSize
	}

	return allVenues, nil
}

// GetInventory retrieves a page of inventory records from OWPROV with extended information.
func (c *client) GetInventory(ctx context.Context, token, reqID, corrID string, limit, offset int, unassigned bool) ([]models.InventoryTag, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	endpoint := fmt.Sprintf("%s/api/v1/inventory?withExtendedInfo=true&limit=%d&offset=%d", baseURL, limit, offset)
	if unassigned {
		endpoint += "&unassigned=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create inventory request", err)
	}

	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV inventory request failed",
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV returned non-200 status for inventory",
				"service", "owprov",
				"status", resp.StatusCode,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var tagResp models.InventoryTagListResponse
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(limitedReader).Decode(&tagResp); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse inventory response", err)
	}

	return tagResp.Tags(), nil
}

// GetInventoryCount retrieves the total count of inventory records accessible to the caller.
func (c *client) GetInventoryCount(ctx context.Context, token, reqID, corrID string, unassigned bool) (int, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return 0, err
	}

	endpoint := fmt.Sprintf("%s/api/v1/inventory?countOnly=true", baseURL)
	if unassigned {
		endpoint += "&unassigned=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "failed to create inventory count request", err)
	}

	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV inventory count request failed",
				"service", "owprov",
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return 0, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV returned non-200 status for inventory count",
				"service", "owprov",
				"status", resp.StatusCode,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return 0, models.ParseResponseError(resp, maxErrorBytes)
	}

	var countResp models.CountOnlyResponse
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(limitedReader).Decode(&countResp); err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "failed to parse inventory count response", err)
	}

	return countResp.Count, nil
}

func (c *client) GetInventorySerials(ctx context.Context, token, reqID, corrID string) ([]string, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	var allSerials []string
	offset := 0
	pages := 0

	for {
		pages++
		endpoint := fmt.Sprintf("%s/api/v1/inventory?serialOnly=true&limit=%d&offset=%d", baseURL, pageSize, offset)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create inventory serials request", err)
		}

		c.setHeaders(req, key, token, reqID, corrID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("downstream OWPROV inventory serials request failed",
					"service", "owprov",
					"error", err,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return nil, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
		}

		if resp.StatusCode != http.StatusOK {
			if c.logger != nil {
				c.logger.Error("downstream OWPROV returned non-200 status for inventory serials",
					"service", "owprov",
					"status", resp.StatusCode,
					"endpoint", req.URL.Path,
					"request_id", reqID,
					"correlation_id", corrID,
				)
			}
			apiErr := models.ParseResponseError(resp, maxErrorBytes)
			resp.Body.Close()
			return nil, apiErr
		}

		var serialsResp models.InventorySerialsResponse
		limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
		err = json.NewDecoder(limitedReader).Decode(&serialsResp)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse inventory serials response", err)
		}

		allSerials = append(allSerials, serialsResp.SerialNumbers...)

		if len(serialsResp.SerialNumbers) < pageSize || pages >= maxPages {
			if len(serialsResp.SerialNumbers) == pageSize && pages >= maxPages && c.logger != nil {
				c.logger.Warn("pagination reached maximum allowed pages", "service", "owprov", "endpoint", "inventory", "max_pages", maxPages, "total_records", len(allSerials))
			}
			break
		}
		offset += pageSize
	}

	return allSerials, nil
}

func normalizeSerial(s string) string {
	clean := strings.ToLower(strings.TrimSpace(s))
	clean = strings.ReplaceAll(clean, ":", "")
	clean = strings.ReplaceAll(clean, "-", "")
	return clean
}

// GetInventoryItem retrieves a single inventory record from OWPROV by serial number.
func (c *client) GetInventoryItem(ctx context.Context, serial, token, reqID, corrID string) (*models.InventoryTag, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	clean := normalizeSerial(serial)

	endpoint := fmt.Sprintf("%s/api/v1/inventory/%s?withExtendedInfo=true", baseURL, url.PathEscape(clean))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create inventory item request", err)
	}

	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV inventory item request failed",
				"service", "owprov",
				"serial", clean,
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "failed to reach downstream owprov service")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, models.NewApiError(http.StatusForbidden, "Forbidden", "Device is not assigned to user or access denied")
	}

	if resp.StatusCode != http.StatusOK {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV returned non-200 status for inventory item",
				"service", "owprov",
				"serial", clean,
				"status", resp.StatusCode,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var tag models.InventoryTag
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(limitedReader).Decode(&tag); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse inventory item response", err)
	}

	return &tag, nil
}

// DeleteInventoryItem removes an inventory item from OWPROV.
func (c *client) DeleteInventoryItem(ctx context.Context, serial, token, reqID, corrID string) error {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return err
	}

	clean := normalizeSerial(serial)
	if clean == "" {
		return models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	endpoint := fmt.Sprintf("%s/api/v1/inventory/%s", baseURL, url.PathEscape(clean))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to create inventory delete request", err)
	}

	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV delete inventory item request failed",
				"service", "owprov",
				"serial", clean,
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "Service unreachable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	return models.ParseResponseError(resp, maxErrorBytes)
}

// ApplyDeviceConfiguration asks OWPROV to compile the effective provisioned
// configuration and push it to OWGW using OWPROV's existing apply flow.
func (c *client) ApplyDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	clean := normalizeSerial(serial)
	if clean == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	endpoint := fmt.Sprintf("%s/api/v1/inventory/%s?applyConfiguration=true", baseURL, url.PathEscape(clean))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create apply configuration request", err)
	}
	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.applyHTTPClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV apply configuration request failed",
				"service", "owprov",
				"serial", clean,
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to read apply configuration response", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWPROV returned an empty apply configuration response")
	}

	var result struct {
		ErrorCode *uint64  `json:"errorCode"`
		Errors    []string `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ErrorCode == nil {
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWPROV returned an invalid apply configuration response")
	}
	if *result.ErrorCode != 0 {
		details := strings.Join(result.Errors, "; ")
		if strings.TrimSpace(details) == "" {
			details = fmt.Sprintf("OWPROV failed to apply the device configuration (errorCode %d)", *result.ErrorCode)
		}
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", details)
	}
	return json.RawMessage(body), nil
}

// AuthorizeDeviceCommand verifies whether the caller is authorized to execute a device command via OWPROV.
func (c *client) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	clean := normalizeSerial(serial)
	if clean == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	if strings.TrimSpace(action) == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "action is required")
	}

	payload := models.DeviceCommandAuthRequest{
		SerialNumber: clean,
		Action:       action,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal authorization request", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/authorization/deviceCommand", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create authorization request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("downstream OWPROV authorization request failed",
				"service", "owprov",
				"serial", clean,
				"action", action,
				"error", err,
				"endpoint", req.URL.Path,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, models.NewApiError(http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var authResult models.DeviceCommandAuthResult
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(limitedReader).Decode(&authResult); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse authorization response", err)
	}

	return &authResult, nil
}

func (c *client) setHeaders(req *http.Request, key, token, reqID, corrID string) {
	if c.internalName != "" {
		req.Header.Set("X-INTERNAL-NAME", c.internalName)
	}
	if key != "" {
		req.Header.Set("X-API-KEY", key)
	}
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
