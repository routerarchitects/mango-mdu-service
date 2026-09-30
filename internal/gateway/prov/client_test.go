package prov

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestListAllEntities(t *testing.T) {
	// Mock server that returns entities in pages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")
		limit, _ := strconv.Atoi(limitStr)
		offset, _ := strconv.Atoi(offsetStr)

		// Mock dataset of 250 entities
		var mockEntities []ProvEntity
		for i := 0; i < 250; i++ {
			mockEntities = append(mockEntities, ProvEntity{
				Info: ProvObjectInfo{
					ID:   fmt.Sprintf("ent-%d", i),
					Name: fmt.Sprintf("Entity %d", i),
				},
				Type: "normal",
			})
		}

		start := offset
		if start > len(mockEntities) {
			start = len(mockEntities)
		}
		end := offset + limit
		if end > len(mockEntities) {
			end = len(mockEntities)
		}

		paginated := mockEntities[start:end]
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProvEntityList{Entities: paginated})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	reqCtx := RequestContext{
		Context:     context.Background(),
		BearerToken: "test-token",
	}

	entities, err := client.ListAllEntities(reqCtx)
	if err != nil {
		t.Fatalf("ListAllEntities failed: %v", err)
	}

	if len(entities) != 250 {
		t.Errorf("expected 250 entities, got %d", len(entities))
	}

	// Validate order and content
	for i, ent := range entities {
		expectedID := fmt.Sprintf("ent-%d", i)
		if ent.Info.ID != expectedID {
			t.Errorf("expected entity ID %s at index %d, got %s", expectedID, i, ent.Info.ID)
		}
	}
}
