package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owsec"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

// PolicyService defines business orchestration operations for policies.
type PolicyService interface {
	GetPolicyOverview(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error)
}

type policyService struct {
	provClient owprov.Client
	secClient  owsec.Client
}

// NewPolicyService creates a new PolicyService with injected gateway clients.
func NewPolicyService(provClient owprov.Client, secClient owsec.Client) PolicyService {
	return &policyService{
		provClient: provClient,
		secClient:  secClient,
	}
}

// GetPolicyOverview orchestrates data retrieval across OWPROV and OWSEC to produce the policy overview.
func (s *policyService) GetPolicyOverview(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
	// 1. Validate UUID syntax
	if _, err := uuid.Parse(policyID); err != nil {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "Invalid policy ID format: must be a valid UUID")
	}

	// 2. Fetch target policy from OWPROV
	policy, err := s.provClient.GetPolicy(ctx, policyID, token, reqID, corrID)
	if err != nil {
		return nil, err
	}

	meta := models.PolicyMetadata{
		ID:          policy.ID,
		Name:        policy.Name,
		Description: policy.Description,
		Entity:      policy.Entity,
		Venue:       policy.Venue,
		Created:     policy.Created,
		Modified:    policy.Modified,
	}

	// 3. Fetch matching roles from OWPROV
	roles, err := s.provClient.GetRolesByPolicy(ctx, policyID, token, reqID, corrID)
	if err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		return &models.PolicyOverviewResponse{
			Policy:                 meta,
			TotalUsers:             0,
			TotalScopedAssignments: 0,
			TotalProperties:        0,
			TotalVenues:            0,
			UsersWithPolicy:        []models.UserWithPolicySummary{},
		}, nil
	}

	// 4. Concurrently or sequentially fetch supporting metadata (Entities, Venues, Users)
	entities, err := s.provClient.GetEntities(ctx, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	entityNameMap := make(map[string]string, len(entities))
	for _, e := range entities {
		entityNameMap[e.ID] = e.Name
	}

	venues, err := s.provClient.GetVenues(ctx, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	venueNameMap := make(map[string]string, len(venues))
	for _, v := range venues {
		venueNameMap[v.ID] = v.Name
	}

	users, err := s.secClient.GetUsers(ctx, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	userMap := make(map[string]models.SecUser, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	// 5. In-memory aggregation
	uniqueEntities := make(map[string]struct{})
	uniqueVenues := make(map[string]struct{})
	totalScopedAssignments := 0

	userSummaries := make(map[string]*models.UserWithPolicySummary)
	var userOrder []string

	for _, role := range roles {
		for _, userID := range role.Users {
			secUser, exists := userMap[userID]
			if !exists {
				// Graceful skip: user not found/accessible in OWSEC
				continue
			}

			totalScopedAssignments++

			if role.Entity != "" {
				uniqueEntities[role.Entity] = struct{}{}
			}
			if role.Venue != "" {
				uniqueVenues[role.Venue] = struct{}{}
			}

			// Resolve entity name
			entityName := entityNameMap[role.Entity]
			if entityName == "" && role.Entity != "" {
				entityName = role.Entity
			}

			// Resolve venue name ("All venues" if empty/null)
			venueName := "All venues"
			if role.Venue != "" {
				if vName, ok := venueNameMap[role.Venue]; ok && vName != "" {
					venueName = vName
				} else {
					venueName = role.Venue
				}
			}

			scopeInfo := models.UserPolicyScopeInfo{
				EntityID:   role.Entity,
				EntityName: entityName,
				VenueID:    role.Venue,
				VenueName:  venueName,
			}

			if summary, ok := userSummaries[userID]; ok {
				summary.ScopedAssignmentsCount++
				summary.Scopes = append(summary.Scopes, scopeInfo)
			} else {
				summary = &models.UserWithPolicySummary{
					ID:                     secUser.ID,
					Name:                   secUser.Name,
					Email:                  secUser.Email,
					UserRole:               secUser.UserRole,
					Avatar:                 secUser.Avatar,
					ScopedAssignmentsCount: 1,
					Scopes:                 []models.UserPolicyScopeInfo{scopeInfo},
				}
				userSummaries[userID] = summary
				userOrder = append(userOrder, userID)
			}
		}
	}

	usersWithPolicy := make([]models.UserWithPolicySummary, 0, len(userOrder))
	for _, uid := range userOrder {
		usersWithPolicy = append(usersWithPolicy, *userSummaries[uid])
	}

	return &models.PolicyOverviewResponse{
		Policy:                 meta,
		TotalUsers:             len(usersWithPolicy),
		TotalScopedAssignments: totalScopedAssignments,
		TotalProperties:        len(uniqueEntities),
		TotalVenues:            len(uniqueVenues),
		UsersWithPolicy:        usersWithPolicy,
	}, nil
}
