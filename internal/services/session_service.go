package services

import (
	"context"
	"strings"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type SessionService struct {
	secClient  *sec.Client
	provClient *prov.Client
}

func NewSessionService(secClient *sec.Client, provClient *prov.Client) *SessionService {
	return &SessionService{
		secClient:  secClient,
		provClient: provClient,
	}
}

type treeHelper struct {
	nodes map[string]treeNodeInfo
}

type treeNodeInfo struct {
	Name     string
	Type     string
	ParentID string
	Path     []models.ScopePathItem
}

func (h *treeHelper) traverse(node prov.ProvTreeNode, parentPath []models.ScopePathItem, parentID string) {
	currentPathItem := models.ScopePathItem{
		ID:   node.UUID,
		Type: node.Type,
		Name: node.Name,
	}
	// Do not include the root system entity in paths, or keep it depending on convention
	var nodePath []models.ScopePathItem
	if node.UUID != "00000000-0000-0000-0000-000000000000" {
		nodePath = append([]models.ScopePathItem{}, parentPath...)
		nodePath = append(nodePath, currentPathItem)
	} else {
		nodePath = parentPath
	}

	h.nodes[node.UUID] = treeNodeInfo{
		Name:     node.Name,
		Type:     node.Type,
		ParentID: parentID,
		Path:     nodePath,
	}

	for _, child := range node.Children {
		h.traverse(child, nodePath, node.UUID)
	}
	for _, venue := range node.Venues {
		h.traverse(venue, nodePath, node.UUID)
	}
}

func (s *SessionService) GetSessionContext(ctx context.Context, token string) (*models.SessionContext, error) {
	// 1. Validate token with owsec
	userInfo, err := s.secClient.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Create request context with the token
	reqCtx := prov.RequestContext{
		Context:     ctx,
		BearerToken: token,
	}

	// 2. Fetch the visible entity/venue tree to construct ancestor paths
	tree, err := s.provClient.GetTree(reqCtx)
	var helper treeHelper
	helper.nodes = make(map[string]treeNodeInfo)
	if err == nil && tree != nil {
		helper.traverse(*tree, nil, "")
	}

	// 3. Fetch all management roles to find assignments for this user
	// Note: We scan through management roles. Under MDU phase 1, roles are created on demand.
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
	var assignments []models.SessionAssignment
	if err == nil {
		for _, r := range roles {
			// Check if user is bound
			userBound := false
			for _, uID := range r.Users {
				if uID == userInfo.ID {
					userBound = true
					break
				}
			}

			if userBound {
				var scopeID string
				var scopeType string
				if r.Entity != "" {
					scopeID = r.Entity
					scopeType = "entity"
				} else if r.Venue != "" {
					scopeID = r.Venue
					scopeType = "venue"
				}

				scopeName := "Unknown Scope"
				var nodePath []models.ScopePathItem
				if info, found := helper.nodes[scopeID]; found {
					scopeName = info.Name
					nodePath = info.Path
				} else {
					// Fallback path
					nodePath = []models.ScopePathItem{
						{
							ID:   scopeID,
							Type: scopeType,
							Name: scopeName,
						},
					}
				}

				assignments = append(assignments, models.SessionAssignment{
					AssignmentID:       r.Info.ID,
					ScopeType:          scopeType,
					ScopeID:            scopeID,
					ScopeName:          scopeName,
					Path:               nodePath,
					Role:               r.Info.Name,
					ManagementRoleID:   r.Info.ID,
					ManagementPolicyID: r.ManagementPolicy,
				})
			}
		}
	}

	// 4. Construct user profile summary
	now := time.Now().UTC()
	userSummary := models.UserSummary{
		ID:          userInfo.ID,
		Name:        userInfo.Name,
		Email:       userInfo.Email,
		Role:        userInfo.UserRole,
		Status:      "active",
		LastLoginAt: &now,
	}

	// 5. Build permissions
	permissions := getEffectivePermissions(userInfo.UserRole)

	// 6. Active scope selection
	var activeScope *models.ScopePathItem
	if len(assignments) > 0 {
		activeScope = &models.ScopePathItem{
			ID:   assignments[0].ScopeID,
			Type: assignments[0].ScopeType,
			Name: assignments[0].ScopeName,
		}
	}

	return &models.SessionContext{
		User:        userSummary,
		ActiveScope: activeScope,
		Assignments: assignments,
		Permissions: permissions,
	}, nil
}

func getEffectivePermissions(role string) models.EffectivePermissionSet {
	switch strings.ToLower(role) {
	case "root", "admin":
		return models.EffectivePermissionSet{
			Hierarchy:      models.RbacDecision{Allowed: true, Mode: "interactive"},
			Users:          models.RbacDecision{Allowed: true, Mode: "interactive"},
			Billing:        models.RbacDecision{Allowed: true, Mode: "interactive"},
			Configurations: models.RbacDecision{Allowed: true, Mode: "interactive"},
			Devices:        models.RbacDecision{Allowed: true, Mode: "interactive"},
		}
	case "installer":
		return models.EffectivePermissionSet{
			Hierarchy:      models.RbacDecision{Allowed: true, Mode: "read_only"},
			Users:          models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Billing:        models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Configurations: models.RbacDecision{Allowed: true, Mode: "interactive"},
			Devices:        models.RbacDecision{Allowed: true, Mode: "interactive"},
		}
	case "csr":
		return models.EffectivePermissionSet{
			Hierarchy:      models.RbacDecision{Allowed: true, Mode: "read_only"},
			Users:          models.RbacDecision{Allowed: true, Mode: "read_only"},
			Billing:        models.RbacDecision{Allowed: true, Mode: "read_only"},
			Configurations: models.RbacDecision{Allowed: true, Mode: "read_only"},
			Devices:        models.RbacDecision{Allowed: true, Mode: "read_only"},
		}
	default:
		return models.EffectivePermissionSet{
			Hierarchy:      models.RbacDecision{Allowed: true, Mode: "read_only"},
			Users:          models.RbacDecision{Allowed: false, Mode: "hidden"},
			Billing:        models.RbacDecision{Allowed: false, Mode: "hidden"},
			Configurations: models.RbacDecision{Allowed: false, Mode: "hidden"},
			Devices:        models.RbacDecision{Allowed: false, Mode: "hidden"},
		}
	}
}
