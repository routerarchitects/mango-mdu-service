package prov

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type RequestContext struct {
	Context       context.Context
	BearerToken   string
	RequestID     string
	CorrelationID string
}

type Client struct {
	discovery    *servicediscovery.Discovery
	httpClient   *http.Client
	internalName string
	BaseURL      string // For testing override
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
	} else {
		// Fallback: Skip verification or use default system trust store if no custom CA is provided
		tlsConfig.InsecureSkipVerify = true
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

func (c *Client) sendRequest(reqCtx RequestContext, method, path string, body io.Reader) (*http.Response, error) {
	var url string
	var apiKey string

	if c.BaseURL != "" {
		url = strings.TrimSuffix(c.BaseURL, "/") + path
		apiKey = "mock-api-key"
	} else {
		if c.discovery == nil {
			return nil, apperror.New(apperror.CodeInternal, "service discovery is not initialized")
		}
		instance := c.discovery.Store().GetServiceInstances("owprov")
		if instance == nil {
			return nil, apperror.New(apperror.CodeNotFound, "owprov service instance not found in discovery store")
		}
		url = strings.TrimSuffix(instance.PublicEndPoint, "/") + path
		apiKey = instance.Key
	}

	req, err := http.NewRequestWithContext(reqCtx.Context, method, url, body)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create http request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X-INTERNAL-NAME", c.internalName)

	if reqCtx.BearerToken != "" {
		req.Header.Set("Authorization", reqCtx.BearerToken)
	}
	if reqCtx.RequestID != "" {
		req.Header.Set("X-Request-Id", reqCtx.RequestID)
	}
	if reqCtx.CorrelationID != "" {
		req.Header.Set("X-Correlation-Id", reqCtx.CorrelationID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "owprov request failed", err)
	}

	return resp, nil
}

type ProvOperator struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Created        int64  `json:"created"`
	Modified       int64  `json:"modified"`
	RegistrationID string `json:"registrationId"`
	EntityID       string `json:"entityId"`
}

func (c *Client) GetOperator(reqCtx RequestContext, operatorID string) (*ProvOperator, error) {
	resp, err := c.sendRequest(reqCtx, http.MethodGet, "/api/v1/operator/"+operatorID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var op ProvOperator
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode operator details", err)
	}

	return &op, nil
}

func (c *Client) UpdateOperator(reqCtx RequestContext, operatorID string, op *ProvOperator) (*ProvOperator, error) {
	bodyBytes, err := json.Marshal(op)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to marshal operator body", err)
	}

	resp, err := c.sendRequest(reqCtx, http.MethodPut, "/api/v1/operator/"+operatorID, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	var updated ProvOperator
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode operator details", err)
	}

	return &updated, nil
}

func (c *Client) DeleteOperator(reqCtx RequestContext, operatorID string) error {
	resp, err := c.sendRequest(reqCtx, http.MethodDelete, "/api/v1/operator/"+operatorID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return apperror.New(apperror.CodeNotFound, "operator not found")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("owprov returned status %d: %s", resp.StatusCode, string(body)))
	}

	return nil
}
