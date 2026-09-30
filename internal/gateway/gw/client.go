package gw

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

type DeviceStatus struct {
	SerialNumber string `json:"serialNumber"`
	Serial       string `json:"serial"`
	Connected    bool   `json:"connected"`
	IP           string `json:"ip"`
	LastSeen     int64  `json:"lastSeen"`
}

func (d DeviceStatus) GetSerial() string {
	if d.SerialNumber != "" {
		return d.SerialNumber
	}
	return d.Serial
}

type Client struct {
	discovery    *servicediscovery.Discovery
	httpClient   *http.Client
	internalName string
	BaseURL      string
	BaseAPIKey   string
}

func NewClient(discovery *servicediscovery.Discovery, tlsRootCA string, internalName string) (*Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	if strings.TrimSpace(tlsRootCA) != "" {
		pemBytes, err := os.ReadFile(tlsRootCA)
		if err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pemBytes) {
				tlsConfig.RootCAs = pool
			}
		}
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

func (c *Client) sendRequest(reqCtx prov.RequestContext, method, path string, body io.Reader) (*http.Response, error) {
	var urlStr string
	var apiKey string
	if c.BaseURL != "" {
		urlStr = strings.TrimSuffix(c.BaseURL, "/") + path
		apiKey = c.BaseAPIKey
		if apiKey == "" {
			apiKey = "mock-api-key"
		}
	} else {
		if c.discovery == nil {
			return nil, apperror.New(apperror.CodeInternal, "service discovery is not initialized")
		}
		instance := c.discovery.Store().GetServiceInstances("owgw")
		if instance == nil {
			return nil, apperror.New(apperror.CodeNotFound, "owgw service instance not found in discovery store")
		}
		urlStr = strings.TrimSuffix(instance.PrivateEndPoint, "/") + path
		apiKey = instance.Key
	}

	req, err := http.NewRequestWithContext(reqCtx.Context, method, urlStr, body)
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
		return nil, apperror.Wrap(apperror.Code("DOWNSTREAM_UNAVAILABLE"), "owgw request failed", err)
	}

	return resp, nil
}

func (c *Client) ListDevicesStatus(reqCtx prov.RequestContext) ([]DeviceStatus, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/devices?deviceWithStatus=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owgw returned status %d: %s", resp.StatusCode, string(body)))
	}

	var res struct {
		Devices []DeviceStatus `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode device status list", err)
	}

	return res.Devices, nil
}

// OWGW Default Configuration & Direct Device Configure Methods

type GWDefaultConfiguration struct {
	Name          string   `json:"name"`
	Model         string   `json:"model"`
	Description   string   `json:"description,omitempty"`
	Configuration string   `json:"configuration"`
	Models        []string `json:"models,omitempty"`
	Created       int64    `json:"created,omitempty"`
	LastModified  int64    `json:"lastModified,omitempty"`
}

type GWDefaultConfigurationList struct {
	Configs []GWDefaultConfiguration `json:"configs"`
}

func (c *Client) ListDefaultConfigurations(reqCtx prov.RequestContext) ([]GWDefaultConfiguration, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/default_configurations", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owgw returned status %d: %s", resp.StatusCode, string(body)))
	}

	var list GWDefaultConfigurationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode default configurations list", err)
	}

	return list.Configs, nil
}

func (c *Client) CreateDefaultConfiguration(reqCtx prov.RequestContext, name string, config interface{}) error {
	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to marshal default config body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/default_configuration/"+name, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owgw returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

func (c *Client) DeleteDefaultConfiguration(reqCtx prov.RequestContext, name string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/default_configuration/"+name, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owgw returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

func (c *Client) ConfigureDevice(reqCtx prov.RequestContext, serialNumber string, config interface{}) error {
	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to marshal device config body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPost, "/api/v1/device/"+serialNumber+"/configure", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owgw returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}

