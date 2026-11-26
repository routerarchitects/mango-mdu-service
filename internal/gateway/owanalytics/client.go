package owanalytics

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
	"strconv"
	"strings"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

const (
	userAgent        = "mango-mdu-service/1.0"
	maxErrorBytes    = 2048
	maxResponseBytes = 10 << 20 // 10 MB
)

// Client defines the contract for communicating with OWANALYTICS.
type Client interface {
	GetBoard(ctx context.Context, id, token, reqID, corrID string) (*models.AnalyticsBoard, error)
	CreateBoard(ctx context.Context, req models.CreateAnalyticsBoardRequest, token, reqID, corrID string) (string, error)
	UpdateBoard(ctx context.Context, id string, req models.CreateAnalyticsBoardRequest, token, reqID, corrID string) error
	DeleteBoard(ctx context.Context, id, token, reqID, corrID string) error
	GetBoardDevices(ctx context.Context, id, token, reqID, corrID string) ([]models.AnalyticsBoardDevice, error)
	GetBoardTimepoints(ctx context.Context, id string, fromDate, toDate int64, token, reqID, corrID string) (json.RawMessage, error)
	GetWifiClients(ctx context.Context, venueID string, token, reqID, corrID string) ([]string, error)
	GetClientLifecycle(ctx context.Context, venueID, clientMac string, fromDate, toDate int64, limit, offset int, token, reqID, corrID string) (json.RawMessage, error)
}

type client struct {
	urlResolver func() string
	httpClient  *http.Client
	logger      *slog.Logger
}

// Config holds configuration for creating an OWANALYTICS client.
type Config struct {
	URLResolver func() string
	Timeout     time.Duration
	TLSConfig   *tls.Config
	Logger      *slog.Logger
}

// NewClient creates a new OWANALYTICS client instance.
func NewClient(cfg Config) Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.TLSConfig != nil {
		transport.TLSClientConfig = cfg.TLSConfig
	}

	return &client{
		urlResolver: cfg.URLResolver,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		logger: cfg.Logger,
	}
}

func (c *client) resolveBaseURL() (string, error) {
	if c.urlResolver == nil {
		return "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owanalytics service endpoint resolver not configured")
	}
	raw := strings.TrimSpace(c.urlResolver())
	if raw == "" {
		return "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owanalytics service endpoint not discovered or available")
	}
	return strings.TrimRight(raw, "/"), nil
}

func (c *client) setHeaders(req *http.Request, token, reqID, corrID string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(token) != "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.Header.Set("Authorization", token)
		}
	}
	if reqID != "" {
		req.Header.Set("X-Request-Id", reqID)
	}
	if corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}
}

func (c *client) GetBoard(ctx context.Context, id, token, reqID, corrID string) (*models.AnalyticsBoard, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/board/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create get board request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var board models.AnalyticsBoard
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&board); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode board response", err)
	}
	return &board, nil
}

func (c *client) CreateBoard(ctx context.Context, req models.CreateAnalyticsBoardRequest, token, reqID, corrID string) (string, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeInternal, "failed to marshal create board request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/board/0", bytes.NewReader(body))
	if err != nil {
		return "", apperror.Wrap(apperror.CodeInternal, "failed to create post board request", err)
	}
	c.setHeaders(httpReq, token, reqID, corrID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", models.ParseResponseError(resp, maxErrorBytes)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return "", apperror.Wrap(apperror.CodeInternal, "failed to decode create board response", err)
	}
	return result.ID, nil
}

func (c *client) UpdateBoard(ctx context.Context, id string, req models.CreateAnalyticsBoardRequest, token, reqID, corrID string) error {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to marshal update board request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/api/v1/board/"+url.PathEscape(id), bytes.NewReader(body))
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to create put board request", err)
	}
	c.setHeaders(httpReq, token, reqID, corrID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.ParseResponseError(resp, maxErrorBytes)
	}
	return nil
}

func (c *client) DeleteBoard(ctx context.Context, id, token, reqID, corrID string) error {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/api/v1/board/"+url.PathEscape(id), nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to create delete board request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return models.ParseResponseError(resp, maxErrorBytes)
	}
	return nil
}

func (c *client) GetBoardDevices(ctx context.Context, id, token, reqID, corrID string) ([]models.AnalyticsBoardDevice, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/board/"+url.PathEscape(id)+"/devices", nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create get board devices request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var response models.AnalyticsBoardDevicesResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&response); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode board devices response", err)
	}
	return response.Devices, nil
}

func (c *client) GetBoardTimepoints(ctx context.Context, id string, fromDate, toDate int64, token, reqID, corrID string) (json.RawMessage, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/v1/board/%s/timepoints?maxRecords=1000", baseURL, url.PathEscape(id))
	if fromDate > 0 {
		endpoint += "&fromDate=" + strconv.FormatInt(fromDate, 10)
	}
	if toDate > 0 {
		endpoint += "&endDate=" + strconv.FormatInt(toDate, 10)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create timepoints request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to read timepoints response", err)
	}
	return json.RawMessage(body), nil
}

func (c *client) GetWifiClients(ctx context.Context, venueID string, token, reqID, corrID string) ([]string, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/v1/wifiClientHistory?macsOnly=true&venue=%s&limit=500", baseURL, url.QueryEscape(venueID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create wifi clients request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	var result struct {
		Entries []string `json:"entries"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode wifi clients response", err)
	}
	return result.Entries, nil
}

func (c *client) GetClientLifecycle(ctx context.Context, venueID, clientMac string, fromDate, toDate int64, limit, offset int, token, reqID, corrID string) (json.RawMessage, error) {
	baseURL, err := c.resolveBaseURL()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	endpoint := fmt.Sprintf("%s/api/v1/wifiClientHistory/%s?venue=%s&limit=%d&offset=%d",
		baseURL, url.PathEscape(clientMac), url.QueryEscape(venueID), limit, offset)
	if fromDate > 0 {
		endpoint += "&fromDate=" + strconv.FormatInt(fromDate, 10)
	}
	if toDate > 0 {
		endpoint += "&endDate=" + strconv.FormatInt(toDate, 10)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create client lifecycle request", err)
	}
	c.setHeaders(req, token, reqID, corrID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "downstream OWANALYTICS service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, models.ParseResponseError(resp, maxErrorBytes)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to read client lifecycle response", err)
	}
	return json.RawMessage(body), nil
}
