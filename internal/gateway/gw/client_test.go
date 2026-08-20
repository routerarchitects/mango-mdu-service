package gw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
)

func TestListDevicesStatus(t *testing.T) {
	t.Parallel()

	client, err := NewClient(nil, "", "mango-mdu-service")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	reqCtx := prov.RequestContext{
		Context: context.Background(),
	}

	if liveBaseURL := os.Getenv("OWGW_BASE_URL"); liveBaseURL != "" {
		client.BaseURL = liveBaseURL
		client.BaseAPIKey = os.Getenv("OWGW_API_KEY")
		reqCtx.BearerToken = os.Getenv("OWGW_BEARER_TOKEN")
		t.Logf("using live OWGW at %s", liveBaseURL)
	} else {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.URL.Path != "/api/v1/devices" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.Header.Get("X-API-KEY"); got != "mock-api-key" {
				t.Fatalf("unexpected X-API-KEY: %q", got)
			}
			if got := r.Header.Get("X-INTERNAL-NAME"); got != "mango-mdu-service" {
				t.Fatalf("unexpected X-INTERNAL-NAME: %q", got)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"devices":[{"serialNumber":"AP-1","connected":true,"ip":"10.0.0.10","lastSeen":1722240000},{"serialNumber":"AP-2","connected":false,"ip":"10.0.0.11","lastSeen":1722240100}]}`))
		}))
		defer server.Close()
		client.BaseURL = server.URL
	}

	devices, err := client.ListDevicesStatus(reqCtx)
	if err != nil {
		t.Fatalf("ListDevicesStatus() error = %v", err)
	}
	t.Logf("parsed devices: %+v", devices)

	if os.Getenv("OWGW_BASE_URL") != "" {
		return
	}

	if len(devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(devices))
	}

	if devices[0].SerialNumber != "AP-1" || !devices[0].Connected || devices[0].IP != "10.0.0.10" || devices[0].LastSeen != 1722240000 {
		t.Fatalf("unexpected first device: %+v", devices[0])
	}

	if devices[1].SerialNumber != "AP-2" || devices[1].Connected || devices[1].IP != "10.0.0.11" || devices[1].LastSeen != 1722240100 {
		t.Fatalf("unexpected second device: %+v", devices[1])
	}
}
