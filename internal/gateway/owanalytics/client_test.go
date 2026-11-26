package owanalytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

func TestOWAnalyticsClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/board/test-board-1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.AnalyticsBoard{
				ID:   "test-board-1",
				Name: "Test Board",
				VenueList: []models.AnalyticsBoardVenue{
					{ID: "venue-1", Name: "Venue One", Interval: 60, Retention: 604800, MonitorSubVenues: true},
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/board/0":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "new-board-uuid"})

		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/board/test-board-1":
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/board/test-board-1":
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/board/test-board-1/devices":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.AnalyticsBoardDevicesResponse{
				Devices: []models.AnalyticsBoardDevice{
					{
						SerialNumber:   "24f5a206aa10",
						Connected:      true,
						DeviceType:     "AP",
						Health:         95,
						Memory:         42,
						Associations2G: 4,
						Associations5G: 8,
						Associations6G: 0,
					},
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wifiClientHistory":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string][]string{
				"entries": {"00:11:22:33:44:55", "aa:bb:cc:dd:ee:ff"},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wifiClientHistory/00:11:22:33:44:55":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"entries": []map[string]any{
					{"timestamp": 1700000000, "rssi": -45, "rx_bytes": 1024, "tx_bytes": 2048},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient(Config{
		URLResolver: func() string { return server.URL },
	})

	ctx := context.Background()

	// 1. GetBoard
	board, err := c.GetBoard(ctx, "test-board-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if board.ID != "test-board-1" || len(board.VenueList) != 1 {
		t.Errorf("Unexpected board response: %+v", board)
	}

	// 2. CreateBoard
	newID, err := c.CreateBoard(ctx, models.CreateAnalyticsBoardRequest{
		Name: "New Board",
		VenueList: []models.AnalyticsBoardVenue{
			{ID: "venue-1", Name: "Venue One", Interval: 60, Retention: 604800, MonitorSubVenues: true},
		},
	}, "token", "req", "corr")
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}
	if newID != "new-board-uuid" {
		t.Errorf("Expected board id 'new-board-uuid', got %q", newID)
	}

	// 3. UpdateBoard
	err = c.UpdateBoard(ctx, "test-board-1", models.CreateAnalyticsBoardRequest{
		Name: "Updated Board",
	}, "token", "req", "corr")
	if err != nil {
		t.Fatalf("UpdateBoard failed: %v", err)
	}

	// 4. DeleteBoard
	err = c.DeleteBoard(ctx, "test-board-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}

	// 5. GetBoardDevices
	devices, err := c.GetBoardDevices(ctx, "test-board-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetBoardDevices failed: %v", err)
	}
	if len(devices) != 1 || devices[0].SerialNumber != "24f5a206aa10" {
		t.Errorf("Unexpected devices response: %+v", devices)
	}

	// 6. GetWifiClients
	clients, err := c.GetWifiClients(ctx, "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetWifiClients failed: %v", err)
	}
	if len(clients) != 2 || clients[0] != "00:11:22:33:44:55" {
		t.Errorf("Unexpected clients response: %+v", clients)
	}

	// 7. GetClientLifecycle
	lifecycleRaw, err := c.GetClientLifecycle(ctx, "venue-1", "00:11:22:33:44:55", 0, 0, 10, 0, "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetClientLifecycle failed: %v", err)
	}
	if len(lifecycleRaw) == 0 {
		t.Errorf("Expected non-empty client lifecycle raw JSON")
	}
}
