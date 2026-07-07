package sec

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type UserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	UserRole string `json:"userRole"`
}

type TokenValidationResponse struct {
	UserInfo UserInfo `json:"userInfo"`
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

func (c *Client) sendRequest(ctx context.Context, method, path string, rawToken string) (*http.Response, error) {
	var urlStr string
	var apiKey string
	if c.BaseURL != "" {
		urlStr = strings.TrimSuffix(c.BaseURL, "/") + path
		apiKey = "mock-api-key"
	} else {
		if c.discovery == nil {
			return nil, apperror.New(apperror.CodeInternal, "service discovery is not initialized")
		}
		instance := c.discovery.Store().GetServiceInstances("owsec")
		if instance == nil {
			return nil, apperror.New(apperror.CodeNotFound, "owsec service instance not found in discovery store")
		}
		urlStr = strings.TrimSuffix(instance.PrivateEndPoint, "/") + path
		apiKey = instance.Key
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create http request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X-INTERNAL-NAME", c.internalName)

	if rawToken != "" {
		if !strings.HasPrefix(strings.ToLower(rawToken), "bearer ") {
			req.Header.Set("Authorization", "Bearer "+rawToken)
		} else {
			req.Header.Set("Authorization", rawToken)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "owsec request failed", err)
	}

	return resp, nil
}

func (c *Client) ValidateToken(ctx context.Context, rawToken string) (*UserInfo, error) {
	token := strings.TrimSpace(rawToken)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return nil, apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	// Validate token
	tokenResp, err := c.sendRequest(ctx, http.MethodGet, "/api/v1/validateToken?token="+url.QueryEscape(token), rawToken)
	if err != nil {
		return nil, err
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode == http.StatusOK {
		var valResp TokenValidationResponse
		if err := json.NewDecoder(tokenResp.Body).Decode(&valResp); err == nil {
			return &valResp.UserInfo, nil
		}
	}

	if tokenResp.StatusCode == http.StatusUnauthorized || tokenResp.StatusCode == http.StatusForbidden || tokenResp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	body, _ := io.ReadAll(tokenResp.Body)
	return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("token validation failed (status=%d): %s", tokenResp.StatusCode, string(body)))
}

// ValidateAPIKey validates a public API key with owsec.
func (c *Client) ValidateAPIKey(ctx context.Context, apiKey string) error {
	tokenResp, err := c.sendRequest(ctx, http.MethodGet, "/api/v1/validateApiKey?apiKey="+url.QueryEscape(apiKey), "")
	if err != nil {
		return err
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode == http.StatusOK {
		return nil
	}

	if tokenResp.StatusCode == http.StatusUnauthorized || tokenResp.StatusCode == http.StatusForbidden {
		return apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	body, _ := io.ReadAll(tokenResp.Body)
	return apperror.New(apperror.CodeInternal, fmt.Sprintf("API key validation failed (status=%d): %s", tokenResp.StatusCode, string(body)))
}

// ClientAdapter adapts *Client to satisfy the auth.PublicAuthValidator interface.
type ClientAdapter struct {
	client *Client
}

// NewClientAdapter wraps a *Client in a ClientAdapter.
func NewClientAdapter(client *Client) auth.PublicAuthValidator {
	return &ClientAdapter{client: client}
}

// ValidateToken validates a public bearer token.
func (a *ClientAdapter) ValidateToken(ctx context.Context, token string) error {
	_, err := a.client.ValidateToken(ctx, token)
	return err
}

// ValidateAPIKey validates a public API key.
func (a *ClientAdapter) ValidateAPIKey(ctx context.Context, apiKey string) error {
	return a.client.ValidateAPIKey(ctx, apiKey)
}

