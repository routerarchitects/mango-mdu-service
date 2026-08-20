package analytics

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type AnalyticsTelemetry struct {
	AirtimeUtilization int                      `json:"airtimeUtilization"`
	NoiseFloorDbm      int                      `json:"noiseFloorDbm"`
	CpuLoadPercent     int                      `json:"cpuLoadPercent"`
	MemoryLoadPercent  int                      `json:"memoryLoadPercent"`
	RssiDistribution   []map[string]interface{} `json:"rssiDistribution"`
}

type VenueTraffic struct {
	VenueID       string `json:"venueId"`
	VenueName     string `json:"venueName"`
	ActiveClients int    `json:"activeClients"`
	UsageGb       string `json:"usageGb"`
	Status        string `json:"status"`
}

type Client struct {
	discovery    *servicediscovery.Discovery
	httpClient   *http.Client
	internalName string
	BaseURL      string
}

func NewClient(discovery *servicediscovery.Discovery, tlsRootCA string, internalName string) (*Client, error) {
	tlsConfig := &tls.Config{}
	if strings.TrimSpace(tlsRootCA) != "" {
		pemBytes, err := os.ReadFile(tlsRootCA)
		if err != nil {
			return nil, fmt.Errorf("read TLS root CA %q: %w", tlsRootCA, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("parse TLS root CA %q: invalid PEM", tlsRootCA)
		}
		tlsConfig.RootCAs = pool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		discovery: discovery,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		internalName: internalName,
	}, nil
}

func (c *Client) sendRequest(reqCtx prov.RequestContext, method, path string) (*http.Response, error) {
	var urlStr string
	var apiKey string
	if c.BaseURL != "" {
		urlStr = strings.TrimSuffix(c.BaseURL, "/") + path
		apiKey = "mock-api-key"
	} else {
		if c.discovery == nil {
			return nil, apperror.New(apperror.CodeInternal, "service discovery is not initialized")
		}
		instance := c.discovery.Store().GetServiceInstances("owanalytics")
		if instance == nil {
			return nil, apperror.New(apperror.CodeNotFound, "owanalytics service instance not found in discovery store")
		}
		urlStr = strings.TrimSuffix(instance.PrivateEndPoint, "/") + path
		apiKey = instance.Key
	}

	req, err := http.NewRequestWithContext(reqCtx.Context, method, urlStr, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create http request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X-INTERNAL-NAME", c.internalName)

	if reqCtx.BearerToken != "" {
		if !strings.HasPrefix(strings.ToLower(reqCtx.BearerToken), "bearer ") {
			req.Header.Set("Authorization", "Bearer "+reqCtx.BearerToken)
		} else {
			req.Header.Set("Authorization", reqCtx.BearerToken)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperror.Wrap(apperror.Code("DOWNSTREAM_UNAVAILABLE"), "owanalytics request failed", err)
	}

	return resp, nil
}

func (c *Client) GetTelemetry(reqCtx prov.RequestContext) (*AnalyticsTelemetry, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/analytics/telemetry")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owanalytics returned status %d: %s", resp.StatusCode, string(body)))
	}

	var res AnalyticsTelemetry
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode telemetry details", err)
	}

	return &res, nil
}

func (c *Client) GetVenueTraffic(reqCtx prov.RequestContext, venueID string) (*VenueTraffic, error) {
	u := fmt.Sprintf("/api/v1/analytics/venue/%s/traffic", venueID)
	resp, err := c.sendRequest(reqCtx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owanalytics venue traffic returned status %d: %s", resp.StatusCode, string(body)))
	}

	var res VenueTraffic
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode venue traffic details", err)
	}

	return &res, nil
}
