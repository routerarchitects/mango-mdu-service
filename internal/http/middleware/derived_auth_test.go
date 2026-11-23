package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
)

func TestCanonicalEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://openwifi.wlan.local:16010", "https://openwifi.wlan.local:16010"},
		{"https://openwifi.wlan.local:16010/", "https://openwifi.wlan.local:16010"},
		{"HTTPS://OPENWIFI.WLAN.LOCAL:16010/", "https://openwifi.wlan.local:16010"},
		{"  https://openwifi.wlan.local:16010/  ", "https://openwifi.wlan.local:16010"},
		{"https://openwifi.wlan.local:16010/api/v1/", "https://openwifi.wlan.local:16010/api/v1"},
		{"", ""},
	}

	for _, tc := range tests {
		actual := middleware.CanonicalEndpoint(tc.input)
		if actual != tc.expected {
			t.Errorf("CanonicalEndpoint(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestDeriveKey_Consistency(t *testing.T) {
	endpoint := "https://openwifi.wlan.local:16010"
	secret := "changeme"

	key1 := middleware.DeriveKey(endpoint, secret)
	key2 := middleware.DeriveKey(endpoint+"/", secret)
	key3 := middleware.DeriveKey("HTTPS://OPENWIFI.WLAN.LOCAL:16010", secret)

	if key1 == "" {
		t.Fatal("DeriveKey produced empty key")
	}
	if key1 != key2 {
		t.Errorf("expected trailing slash to produce same key: %s vs %s", key1, key2)
	}
	if key1 != key3 {
		t.Errorf("expected case insensitive endpoint to produce same key: %s vs %s", key1, key3)
	}

	diffSecret := middleware.DeriveKey(endpoint, "different-secret")
	if diffSecret == key1 {
		t.Error("different secret must produce different derived key")
	}

	diffEndpoint := middleware.DeriveKey("https://other.wlan.local:16010", secret)
	if diffEndpoint == key1 {
		t.Error("different endpoint must produce different derived key")
	}
}

func TestVerifyAPIKey(t *testing.T) {
	endpoint := "https://openwifi.wlan.local:16010"
	secret := "changeme"
	discoveryKey := middleware.DeriveKey(endpoint, secret)

	// 1. Matches when providing the raw secret
	if !middleware.VerifyAPIKey("changeme", endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to succeed with raw secret")
	}

	// 2. Matches when providing the derived discovery key directly
	if !middleware.VerifyAPIKey(discoveryKey, endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to succeed with derived discovery key")
	}

	// 3. Rejects incorrect API key
	if middleware.VerifyAPIKey("wrong-key", endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to fail with wrong key")
	}

	// 4. Rejects empty API key
	if middleware.VerifyAPIKey("", endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to fail with empty key")
	}

	// 5. Rejects when derived with different secret
	diffKey := middleware.DeriveKey(endpoint, "other-secret")
	if middleware.VerifyAPIKey(diffKey, endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to reject key derived with different secret")
	}

	// 6. Rejects when derived with different public endpoint
	diffEpKey := middleware.DeriveKey("https://other:16010", secret)
	if middleware.VerifyAPIKey(diffEpKey, endpoint, secret, discoveryKey) {
		t.Error("expected VerifyAPIKey to reject key derived with different endpoint")
	}
}

func TestRequireDerivedInternalAPIKey_Middleware(t *testing.T) {
	endpoint := "https://openwifi.wlan.local:16010"
	secret := "changeme"

	mw, err := middleware.RequireDerivedInternalAPIKey(middleware.DerivedInternalAPIKeyConfig{
		PublicEndpoint:   endpoint,
		ConfiguredSecret: secret,
	})
	if err != nil {
		t.Fatalf("failed to create middleware: %v", err)
	}

	app := fiber.New()
	app.Get("/private/test", mw, func(c fiber.Ctx) error {
		internalName := c.Locals("internal_name")
		return c.JSON(fiber.Map{"internalName": internalName})
	})

	// Case 1: Missing X-INTERNAL-NAME
	req1 := httptest.NewRequest(http.MethodGet, "/private/test", nil)
	req1.Header.Set("X-API-KEY", "changeme")
	resp1, err := app.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-INTERNAL-NAME, got status=%d err=%v", resp1.StatusCode, err)
	}

	// Case 2: Missing X-API-KEY
	req2 := httptest.NewRequest(http.MethodGet, "/private/test", nil)
	req2.Header.Set("X-INTERNAL-NAME", "mcp-server")
	resp2, err := app.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-API-KEY, got status=%d err=%v", resp2.StatusCode, err)
	}

	// Case 3: Invalid X-API-KEY
	req3 := httptest.NewRequest(http.MethodGet, "/private/test", nil)
	req3.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req3.Header.Set("X-API-KEY", "bad-key")
	resp3, err := app.Test(req3)
	if err != nil || resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid X-API-KEY, got status=%d err=%v", resp3.StatusCode, err)
	}

	// Case 4: Valid with raw secret
	req4 := httptest.NewRequest(http.MethodGet, "/private/test", nil)
	req4.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req4.Header.Set("X-API-KEY", "changeme")
	resp4, err := app.Test(req4)
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for valid raw secret, got status=%d err=%v", resp4.StatusCode, err)
	}

	// Case 5: Valid with derived key
	derivedKey := middleware.DeriveKey(endpoint, secret)
	req5 := httptest.NewRequest(http.MethodGet, "/private/test", nil)
	req5.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req5.Header.Set("X-API-KEY", derivedKey)
	resp5, err := app.Test(req5)
	if err != nil || resp5.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for valid derived key, got status=%d err=%v", resp5.StatusCode, err)
	}
}
