package services_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

type mockProvClient struct {
	getPolicyFn        func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error)
	getRolesByPolicyFn func(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error)
	getEntitiesFn      func(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error)
	getVenuesFn        func(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error)
}

func (m *mockProvClient) GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
	if m.getPolicyFn != nil {
		return m.getPolicyFn(ctx, id, token, reqID, corrID)
	}
	return nil, nil
}

func (m *mockProvClient) GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
	if m.getRolesByPolicyFn != nil {
		return m.getRolesByPolicyFn(ctx, policyID, token, reqID, corrID)
	}
	return nil, nil
}

func (m *mockProvClient) GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
	if m.getEntitiesFn != nil {
		return m.getEntitiesFn(ctx, token, reqID, corrID)
	}
	return nil, nil
}

func (m *mockProvClient) GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
	if m.getVenuesFn != nil {
		return m.getVenuesFn(ctx, token, reqID, corrID)
	}
	return nil, nil
}

type mockSecClient struct {
	getUsersFn func(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error)
}

func (m *mockSecClient) GetUsers(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error) {
	if m.getUsersFn != nil {
		return m.getUsersFn(ctx, token, reqID, corrID)
	}
	return nil, nil
}

func TestGetPolicyOverview_MalformedUUID_TC_POL_009(t *testing.T) {
	svc := services.NewPolicyService(&mockProvClient{}, &mockSecClient{})

	_, err := svc.GetPolicyOverview(context.Background(), "invalid-uuid", "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error for malformed UUID, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok {
		t.Fatalf("expected models.ApiError, got %T", err)
	}
	if apiErr.ErrorCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", apiErr.ErrorCode)
	}
}

func TestGetPolicyOverview_PolicyNotFound_TC_POL_003(t *testing.T) {
	prov := &mockProvClient{
		getPolicyFn: func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
			return nil, models.NewApiError(http.StatusNotFound, "Not Found", "Management policy not found")
		},
	}
	svc := services.NewPolicyService(prov, &mockSecClient{})

	validUUID := "523e4567-e89b-12d3-a456-426614174000"
	_, err := svc.GetPolicyOverview(context.Background(), validUUID, "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok || apiErr.ErrorCode != http.StatusNotFound {
		t.Errorf("expected 404 ApiError, got %+v", err)
	}
}

func TestGetPolicyOverview_UnassignedPolicy_TC_POL_002(t *testing.T) {
	validUUID := "523e4567-e89b-12d3-a456-426614174000"
	prov := &mockProvClient{
		getPolicyFn: func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
			return &models.ManagementPolicy{
				ID:          validUUID,
				Name:        "Empty Policy",
				Description: "No roles assigned",
			}, nil
		},
		getRolesByPolicyFn: func(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
			return []models.ManagementRole{}, nil
		},
	}
	svc := services.NewPolicyService(prov, &mockSecClient{})

	res, err := svc.GetPolicyOverview(context.Background(), validUUID, "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalUsers != 0 || res.TotalScopedAssignments != 0 || res.TotalProperties != 0 || res.TotalVenues != 0 {
		t.Errorf("expected all counters to be 0, got %+v", res)
	}
	if len(res.UsersWithPolicy) != 0 {
		t.Errorf("expected empty usersWithPolicy, got %+v", res.UsersWithPolicy)
	}
}

func TestGetPolicyOverview_ActiveMultiScope_TC_POL_001(t *testing.T) {
	policyID := "523e4567-e89b-12d3-a456-426614174000"
	entityID := "e290f1ee-6c54-4b01-90e6-d701748f0851"
	venue1ID := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	venue2ID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	user1ID := "4b96f542-2304-4c15-b19e-f43d21d93c50"
	user2ID := "11111111-2222-3333-4444-555555555555"

	prov := &mockProvClient{
		getPolicyFn: func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
			return &models.ManagementPolicy{
				ID:   policyID,
				Name: "Network Operator",
			}, nil
		},
		getRolesByPolicyFn: func(ctx context.Context, pID, token, reqID, corrID string) ([]models.ManagementRole, error) {
			return []models.ManagementRole{
				{
					ID:               "role-1",
					ManagementPolicy: policyID,
					Entity:           entityID,
					Venue:            venue1ID,
					Users:            []string{user1ID},
				},
				{
					ID:               "role-2",
					ManagementPolicy: policyID,
					Entity:           entityID,
					Venue:            venue2ID,
					Users:            []string{user1ID},
				},
				{
					ID:               "role-3",
					ManagementPolicy: policyID,
					Entity:           entityID,
					Venue:            "", // Property-wide: "All venues"
					Users:            []string{user2ID},
				},
			}, nil
		},
		getEntitiesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
			return []models.Entity{
				{ID: entityID, Name: "Sunrise Apartments"},
			}, nil
		},
		getVenuesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
			return []models.Venue{
				{ID: venue1ID, Name: "Tower A"},
				{ID: venue2ID, Name: "Tower B"},
			}, nil
		},
	}

	sec := &mockSecClient{
		getUsersFn: func(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error) {
			return []models.SecUser{
				{ID: user1ID, Name: "User One", Email: "u1@example.com", UserRole: "noc"},
				{ID: user2ID, Name: "User Two", Email: "u2@example.com", UserRole: "admin"},
			}, nil
		},
	}

	svc := services.NewPolicyService(prov, sec)
	res, err := svc.GetPolicyOverview(context.Background(), policyID, "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalUsers != 2 {
		t.Errorf("expected TotalUsers=2, got %d", res.TotalUsers)
	}
	if res.TotalScopedAssignments != 3 {
		t.Errorf("expected TotalScopedAssignments=3, got %d", res.TotalScopedAssignments)
	}
	if res.TotalProperties != 1 {
		t.Errorf("expected TotalProperties=1, got %d", res.TotalProperties)
	}
	if res.TotalVenues != 2 {
		t.Errorf("expected TotalVenues=2, got %d", res.TotalVenues)
	}

	// Verify User 1 has 2 scopes
	if len(res.UsersWithPolicy[0].Scopes) != 2 {
		t.Fatalf("expected 2 scopes for user 1, got %d", len(res.UsersWithPolicy[0].Scopes))
	}
	if res.UsersWithPolicy[0].Scopes[0].VenueName != "Tower A" || res.UsersWithPolicy[0].Scopes[1].VenueName != "Tower B" {
		t.Errorf("unexpected venue names: %+v", res.UsersWithPolicy[0].Scopes)
	}

	// Verify User 2 has 1 scope with "All venues"
	if len(res.UsersWithPolicy[1].Scopes) != 1 {
		t.Fatalf("expected 1 scope for user 2, got %d", len(res.UsersWithPolicy[1].Scopes))
	}
	if res.UsersWithPolicy[1].Scopes[0].VenueName != "All venues" {
		t.Errorf("expected 'All venues', got %q", res.UsersWithPolicy[1].Scopes[0].VenueName)
	}
}

func TestGetPolicyOverview_UnmatchedUser_TC_POL_010(t *testing.T) {
	policyID := "523e4567-e89b-12d3-a456-426614174000"
	userMatched := "4b96f542-2304-4c15-b19e-f43d21d93c50"
	userDeleted := "99999999-9999-9999-9999-999999999999"

	prov := &mockProvClient{
		getPolicyFn: func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
			return &models.ManagementPolicy{ID: policyID, Name: "Test Policy"}, nil
		},
		getRolesByPolicyFn: func(ctx context.Context, pID, token, reqID, corrID string) ([]models.ManagementRole, error) {
			return []models.ManagementRole{
				{ID: "r1", ManagementPolicy: policyID, Entity: "e1", Users: []string{userMatched}},
				{ID: "r2", ManagementPolicy: policyID, Entity: "e2", Users: []string{userDeleted}},
			}, nil
		},
		getEntitiesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
			return []models.Entity{{ID: "e1", Name: "Prop 1"}, {ID: "e2", Name: "Prop 2"}}, nil
		},
		getVenuesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
			return []models.Venue{}, nil
		},
	}

	sec := &mockSecClient{
		getUsersFn: func(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error) {
			// Only returns userMatched, userDeleted is missing
			return []models.SecUser{
				{ID: userMatched, Name: "Matched User"},
			}, nil
		},
	}

	svc := services.NewPolicyService(prov, sec)
	res, err := svc.GetPolicyOverview(context.Background(), policyID, "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalUsers != 1 {
		t.Errorf("expected TotalUsers=1, got %d", res.TotalUsers)
	}
	if res.TotalProperties != 1 {
		t.Errorf("expected TotalProperties=1 (excluding unmatched user's property), got %d", res.TotalProperties)
	}
	if len(res.UsersWithPolicy) != 1 {
		t.Errorf("expected 1 user in usersWithPolicy, got %d", len(res.UsersWithPolicy))
	}
	if res.UsersWithPolicy[0].ID != userMatched {
		t.Errorf("expected matched user, got %s", res.UsersWithPolicy[0].ID)
	}
}

func TestGetPolicyOverview_MultiUserRole_TC_POL_011(t *testing.T) {
	policyID := "523e4567-e89b-12d3-a456-426614174000"
	u1 := "11111111-1111-1111-1111-111111111111"
	u2 := "22222222-2222-2222-2222-222222222222"

	prov := &mockProvClient{
		getPolicyFn: func(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
			return &models.ManagementPolicy{ID: policyID, Name: "Test Policy"}, nil
		},
		getRolesByPolicyFn: func(ctx context.Context, pID, token, reqID, corrID string) ([]models.ManagementRole, error) {
			// Single role containing TWO users
			return []models.ManagementRole{
				{ID: "r1", ManagementPolicy: policyID, Entity: "e1", Users: []string{u1, u2}},
			}, nil
		},
		getEntitiesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
			return []models.Entity{{ID: "e1", Name: "Prop 1"}}, nil
		},
		getVenuesFn: func(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
			return []models.Venue{}, nil
		},
	}

	sec := &mockSecClient{
		getUsersFn: func(ctx context.Context, token, reqID, corrID string) ([]models.SecUser, error) {
			return []models.SecUser{
				{ID: u1, Name: "Alice"},
				{ID: u2, Name: "Bob"},
			}, nil
		},
	}

	svc := services.NewPolicyService(prov, sec)
	res, err := svc.GetPolicyOverview(context.Background(), policyID, "token", "req", "corr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalUsers != 2 {
		t.Errorf("expected TotalUsers=2, got %d", res.TotalUsers)
	}
	if res.TotalScopedAssignments != 2 {
		t.Errorf("expected TotalScopedAssignments=2, got %d", res.TotalScopedAssignments)
	}
	if len(res.UsersWithPolicy) != 2 {
		t.Errorf("expected 2 users in usersWithPolicy, got %d", len(res.UsersWithPolicy))
	}
}
