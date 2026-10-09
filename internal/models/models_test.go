package models_test

import (
	"encoding/json"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

func TestPolicyOverviewResponseSerialization(t *testing.T) {
	resp := models.PolicyOverviewResponse{
		Policy: models.PolicyMetadata{
			ID:          "523e4567-e89b-12d3-a456-426614174000",
			Name:        "Network Operator",
			Description: "Monitor devices and manage network configuration.",
			Entity:      "",
			Venue:       "",
			Created:     1725000000,
			Modified:    1725500000,
		},
		TotalUsers:             2,
		TotalScopedAssignments: 3,
		TotalProperties:        1,
		TotalVenues:            2,
		UsersWithPolicy: []models.UserWithPolicySummary{
			{
				ID:                     "4b96f542-2304-4c15-b19e-f43d21d93c50",
				Name:                   "Anita Sharma",
				Email:                  "anita@ipnx.example",
				UserRole:               "noc",
				Avatar:                 "1",
				ScopedAssignmentsCount: 2,
				Scopes: []models.UserPolicyScopeInfo{
					{
						EntityID:   "e290f1ee-6c54-4b01-90e6-d701748f0851",
						EntityName: "Sunrise Apartments",
						VenueID:    "f47ac10b-58cc-4372-a567-0e02b2c3d479",
						VenueName:  "Tower A",
					},
					{
						EntityID:   "e290f1ee-6c54-4b01-90e6-d701748f0851",
						EntityName: "Sunrise Apartments",
						VenueID:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						VenueName:  "Tower B",
					},
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal PolicyOverviewResponse: %v", err)
	}

	var parsed models.PolicyOverviewResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal PolicyOverviewResponse: %v", err)
	}

	if parsed.TotalUsers != 2 {
		t.Errorf("expected TotalUsers=2, got %d", parsed.TotalUsers)
	}
	if parsed.TotalScopedAssignments != 3 {
		t.Errorf("expected TotalScopedAssignments=3, got %d", parsed.TotalScopedAssignments)
	}
	if parsed.Policy.Name != "Network Operator" {
		t.Errorf("expected policy name 'Network Operator', got %q", parsed.Policy.Name)
	}
	if len(parsed.UsersWithPolicy) != 1 || len(parsed.UsersWithPolicy[0].Scopes) != 2 {
		t.Errorf("unexpected users or scopes count: %+v", parsed.UsersWithPolicy)
	}
}

func TestApiError(t *testing.T) {
	errWithDetails := models.NewApiError(400, "Bad Request", "Malformed UUID")
	if errWithDetails.Error() != "400 Bad Request: Malformed UUID" {
		t.Errorf("unexpected error string: %s", errWithDetails.Error())
	}

	errWithoutDetails := models.NewApiError(404, "Not Found")
	if errWithoutDetails.Error() != "404 Not Found" {
		t.Errorf("unexpected error string: %s", errWithoutDetails.Error())
	}

	data, err := json.Marshal(errWithDetails)
	if err != nil {
		t.Fatalf("failed to marshal ApiError: %v", err)
	}

	var parsed models.ApiError
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal ApiError: %v", err)
	}

	if parsed.ErrorCode != 400 || parsed.ErrorDescription != "Bad Request" || parsed.ErrorDetails != "Malformed UUID" {
		t.Errorf("unexpected parsed ApiError: %+v", parsed)
	}
}
