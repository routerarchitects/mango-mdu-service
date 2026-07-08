package services

import (
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

func (s *SessionService) GetSessionContext(reqCtx prov.RequestContext) (*models.SessionContext, error) {
	// 1. Validate token with owsec
	userInfo, err := s.secClient.ValidateToken(reqCtx.Context, reqCtx.BearerToken)
	if err != nil {
		return nil, err
	}

	// 2. Fetch the visible entity/venue tree to construct ancestor paths
	tree, err := s.provClient.GetTree(reqCtx)
	if err != nil {
		return nil, err
	}
	var helper treeHelper
	helper.nodes = make(map[string]treeNodeInfo)
	if tree != nil {
		helper.traverse(*tree, nil, "")
	}

	// 3. Fetch all management roles to find assignments for this user
	// Note: We scan through management roles. Under MDU phase 1, roles are created on demand.
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
	if err != nil {
		return nil, err
	}
	var assignments []models.SessionAssignment
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

	// 5. Build permissions dynamically from active assignment policy in PROV
	var permissions models.EffectivePermissionSet
	if len(assignments) > 0 {
		activePolicyID := assignments[0].ManagementPolicyID
		policy, err := s.provClient.GetPolicy(reqCtx, activePolicyID)
		if err != nil {
			return nil, err
		}
		permissions = derivePermissionsFromPolicy(policy)
	} else {
		// If no assignments are active, permissions are completely hidden/denied.
		// PROV is the only source of truth for RBAC.
		permissions = models.EffectivePermissionSet{
			Hierarchy:      models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Users:          models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Billing:        models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Configurations: models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
			Devices:        models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
		}
	}

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

func derivePermissionsFromPolicy(policy *prov.ProvManagementPolicy) models.EffectivePermissionSet {
	res := models.EffectivePermissionSet{
		Hierarchy:      models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
		Users:          models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
		Billing:        models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
		Configurations: models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
		Devices:        models.RbacDecision{Allowed: false, Mode: "hidden", Reason: "Insufficient privileges"},
	}

	if policy == nil {
		return res
	}

	checkAccess := func(targetResources []string) (allowed bool, mode string) {
		hasRead := false
		hasWrite := false
		for _, entry := range policy.Entries {
			match := false
			for _, r := range entry.Resources {
				for _, tr := range targetResources {
					if r == tr {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
			if match {
				for _, acc := range entry.Access {
					if acc == "MODIFY" || acc == "DELETE" {
						hasWrite = true
					} else if acc == "READ" {
						hasRead = true
					}
				}
			}
		}
		if hasWrite {
			return true, "interactive"
		}
		if hasRead {
			return true, "read_only"
		}
		return false, "hidden"
	}

	// Hierarchy
	if allowed, mode := checkAccess([]string{"entity", "venue"}); allowed {
		res.Hierarchy = models.RbacDecision{Allowed: true, Mode: mode}
	}
	// Users
	if allowed, mode := checkAccess([]string{"operator", "managementRole", "managementPolicy"}); allowed {
		res.Users = models.RbacDecision{Allowed: true, Mode: mode}
	}
	// Billing
	if allowed, mode := checkAccess([]string{"billing"}); allowed {
		res.Billing = models.RbacDecision{Allowed: true, Mode: mode}
	}
	// Configurations
	if allowed, mode := checkAccess([]string{"configuration"}); allowed {
		res.Configurations = models.RbacDecision{Allowed: true, Mode: mode}
	}
	// Devices
	if allowed, mode := checkAccess([]string{"inventory"}); allowed {
		res.Devices = models.RbacDecision{Allowed: true, Mode: mode}
	}

	return res
}
