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

	"github.com/google/uuid"
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
	AuthEnabled  bool
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
		AuthEnabled:  true,
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
		return nil, apperror.Wrap(apperror.Code("DOWNSTREAM_UNAVAILABLE"), "owsec request failed", err)
	}

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), fmt.Sprintf("owsec returned status %d: %s", resp.StatusCode, string(body)))
	}

	return resp, nil
}

func (c *Client) ValidateToken(ctx context.Context, rawToken string) (*UserInfo, error) {
	if !c.AuthEnabled {
		userID := "00000000-0000-0000-0000-000000000000"
		userEmail := "mdu-admin@example.com"
		userName := "MDU Admin"
		userRole := "admin"

		token := strings.TrimSpace(rawToken)
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = strings.TrimSpace(token[7:])
		}
		if token != "" && !strings.Contains(token, ".") {
			userID = token
			userName = "Mock User " + token
			userEmail = token + "@example.com"
		}

		return &UserInfo{
			ID:       userID,
			Email:    userEmail,
			Name:     userName,
			Owner:    "00000000-0000-0000-0000-000000000000",
			UserRole: userRole,
		}, nil
	}

	token := strings.TrimSpace(rawToken)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return nil, apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	// Validate token via OWSEC's /validateToken endpoint.
	//
	// OWSEC's handler reads the token exclusively from the ?token= query parameter
	// (Poco::URI::getQueryParameters in the C++ source), not from the Authorization
	// header. This is a constraint of the upstream OWSEC contract.
	//
	// Security context: this call targets OWSEC's PRIVATE endpoint
	// (instance.PrivateEndPoint), which is an internal service-to-service link
	// protected by X-API-KEY + X-INTERNAL-NAME authentication and TLS encryption.
	// The query string is never exposed to public reverse proxies, external access
	// logs, or untrusted network observers.
	tokenResp, err := c.sendRequest(ctx, http.MethodGet, "/api/v1/validateToken?token="+url.QueryEscape(token), rawToken)
	if err != nil {
		return nil, err
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode == http.StatusOK {
		var valResp TokenValidationResponse
		if err := json.NewDecoder(tokenResp.Body).Decode(&valResp); err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode validateToken response (downstream contract drift)", err)
		}
		return &valResp.UserInfo, nil
	}

	if tokenResp.StatusCode == http.StatusUnauthorized ||
		tokenResp.StatusCode == http.StatusForbidden ||
		tokenResp.StatusCode == http.StatusNotFound ||
		tokenResp.StatusCode == http.StatusBadRequest {
		return nil, apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	body, _ := io.ReadAll(tokenResp.Body)
	return nil, apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), fmt.Sprintf("token validation failed (status=%d): %s", tokenResp.StatusCode, string(body)))
}

// GetUser fetches user information from owsec by user ID.
func (c *Client) GetUser(ctx context.Context, userID string, rawToken string) (*UserInfo, error) {
	// First validate that userID is a valid UUID
	if _, err := uuid.Parse(userID); err != nil {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid user identifier format")
	}

	if !c.AuthEnabled {
		// Mock responses for testing
		if userID == "00000000-0000-0000-0000-000000000004" || userID == "00000000-0000-0000-0000-ffffffffffff" {
			return nil, apperror.New(apperror.CodeNotFound, "user not found")
		}
		return &UserInfo{
			ID:       userID,
			Email:    userID + "@example.com",
			Name:     "Mock User " + userID,
			Owner:    "00000000-0000-0000-0000-000000000000",
			UserRole: "admin",
		}, nil
	}

	resp, err := c.sendRequest(ctx, http.MethodGet, "/api/v1/user/"+url.PathEscape(userID), rawToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var user UserInfo
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode user info", err)
		}
		return &user, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodeNotFound, "user not found")
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, apperror.New(apperror.CodeUnauthorized, "unauthorized to read user info")
	}

	body, _ := io.ReadAll(resp.Body)
	return nil, apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), fmt.Sprintf("failed to get user (status=%d): %s", resp.StatusCode, string(body)))
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

	if tokenResp.StatusCode == http.StatusUnauthorized ||
		tokenResp.StatusCode == http.StatusForbidden ||
		tokenResp.StatusCode == http.StatusNotFound ||
		tokenResp.StatusCode == http.StatusBadRequest {
		return apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	body, _ := io.ReadAll(tokenResp.Body)
	return apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), fmt.Sprintf("API key validation failed (status=%d): %s", tokenResp.StatusCode, string(body)))
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

type UserListResponse struct {
	Users []UserInfo `json:"users"`
}

func (c *Client) ListUsers(ctx context.Context, bearerToken string) ([]UserInfo, error) {
	resp, err := c.sendRequest(ctx, http.MethodGet, "/api/v1/users", bearerToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf("owsec users returned status %d: %s", resp.StatusCode, string(body)))
	}

	var res UserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to decode user list", err)
	}

	return res.Users, nil
}
