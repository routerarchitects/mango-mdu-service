package owprov

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const propertyInventorySelectBatchSize = 100

type scopeInventoryReferences struct {
	Devices []string `json:"devices"`
}

// GetPropertyTree retrieves the caller's authorization-filtered OWPROV tree.
func (c *client) GetPropertyTree(ctx context.Context, token, reqID, corrID string) (*models.OWProvTreeNode, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/entity?getTree=true", nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create property tree request", err)
	}
	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var node models.OWProvTreeNode
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&node); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse property tree response", err)
	}
	return &node, nil
}

// GetInventoryForScope resolves the inventory UUIDs explicitly assigned to one
// Entity or Venue. OWPROV stores those references on the scope record, so MDU
// resolves them through inventory select requests before using device serials.
// Descendant expansion remains orchestrated by MDU.
func (c *client) GetInventoryForScope(ctx context.Context, entityID, venueID, token, reqID, corrID string) ([]models.InventoryTag, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}
	if (entityID == "") == (venueID == "") {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "exactly one inventory scope is required")
	}

	resource, scopeID := "venue", venueID
	if entityID != "" {
		resource, scopeID = "entity", entityID
	}
	scopeURL := baseURL + "/api/v1/" + resource + "/" + url.PathEscape(scopeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scopeURL, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create scope inventory reference request", err)
	}
	c.setHeaders(req, key, token, reqID, corrID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := models.ParseResponseError(resp, maxErrorBytes)
		resp.Body.Close()
		return nil, apiErr
	}
	var references scopeInventoryReferences
	err = json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&references)
	resp.Body.Close()
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse scope inventory references", err)
	}

	ids := make([]string, 0, len(references.Devices))
	wanted := make(map[string]struct{}, len(references.Devices))
	for _, rawID := range references.Devices {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := wanted[id]; exists {
			continue
		}
		wanted[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []models.InventoryTag{}, nil
	}

	all := make([]models.InventoryTag, 0, len(ids))
	resolved := make(map[string]struct{}, len(ids))
	for start := 0; start < len(ids); start += propertyInventorySelectBatchSize {
		end := start + propertyInventorySelectBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		params := url.Values{
			"withExtendedInfo": {"true"},
			"select":           {strings.Join(ids[start:end], ",")},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/inventory?"+params.Encode(), nil)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to create scoped inventory request", err)
		}
		c.setHeaders(req, key, token, reqID, corrID)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
		}
		if resp.StatusCode != http.StatusOK {
			apiErr := models.ParseResponseError(resp, maxErrorBytes)
			resp.Body.Close()
			return nil, apiErr
		}

		var envelope models.InventoryTagListResponse
		err = json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&envelope)
		resp.Body.Close()
		if err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to parse scoped inventory response", err)
		}
		batch := envelope.Tags()
		for _, tag := range batch {
			id := strings.TrimSpace(tag.ID)
			if _, expected := wanted[id]; !expected {
				continue
			}
			if _, duplicate := resolved[id]; duplicate {
				continue
			}
			resolved[id] = struct{}{}
			all = append(all, tag)
		}
	}
	if len(resolved) != len(wanted) {
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWPROV returned incomplete scoped inventory")
	}
	return all, nil
}

// GetVenue retrieves a single venue record by ID from OWPROV.
func (c *client) GetVenue(ctx context.Context, id, token, reqID, corrID string) (*models.VenueDetails, error) {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/v1/venue/%s", baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create get venue request", err)
	}
	c.setHeaders(req, key, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var venue models.VenueDetails
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&venue); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode venue response", err)
	}
	return &venue, nil
}

// UpdateVenue updates a venue record by ID in OWPROV.
func (c *client) UpdateVenue(ctx context.Context, id string, body []byte, token, reqID, corrID string) error {
	baseURL, key, err := c.resolveTarget()
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/v1/venue/%s", baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to create update venue request", err)
	}
	c.setHeaders(req, key, token, reqID, corrID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWPROV service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.ParseResponseError(resp, maxErrorBytes)
	}
	return nil
}
