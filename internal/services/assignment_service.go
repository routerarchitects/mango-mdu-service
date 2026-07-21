package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type AssignmentService struct {
	provClient *prov.Client
	secClient  *sec.Client
}

func NewAssignmentService(provClient *prov.Client, secClient *sec.Client) *AssignmentService {
	return &AssignmentService{
		provClient: provClient,
		secClient:  secClient,
	}
}

func (s *AssignmentService) getLookupMaps(reqCtx prov.RequestContext) (map[string]prov.ProvTreeNode, map[string]treeNodeInfo, map[string]prov.ProvEntity, map[string]prov.ProvVenue, error) {
	tree, err := s.provClient.GetTree(reqCtx)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	nodeMap := make(map[string]prov.ProvTreeNode)
	nodePathMap := make(map[string]treeNodeInfo)

	var traverse func(node prov.ProvTreeNode, parentPath []models.ScopePathItem, parentID string)
	traverse = func(node prov.ProvTreeNode, parentPath []models.ScopePathItem, parentID string) {
		nodeMap[node.UUID] = node

		currentPathItem := models.ScopePathItem{
			ID:   node.UUID,
			Type: node.Type,
			Name: node.Name,
		}
		var nodePath []models.ScopePathItem
		if node.UUID != "00000000-0000-0000-0000-000000000000" {
			nodePath = append([]models.ScopePathItem{}, parentPath...)
			nodePath = append(nodePath, currentPathItem)
		} else {
			nodePath = parentPath
		}

		nodePathMap[node.UUID] = treeNodeInfo{
			Name:     node.Name,
			Type:     node.Type,
			ParentID: parentID,
			Path:     nodePath,
		}

		for _, child := range node.Children {
			traverse(child, nodePath, node.UUID)
		}
		for _, venue := range node.Venues {
			traverse(venue, nodePath, node.UUID)
		}
	}
	if tree != nil {
		traverse(*tree, nil, "")
	}

	entities, err := s.provClient.ListAllEntities(reqCtx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	venues, err := s.provClient.ListAllVenues(reqCtx)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	entityMap := make(map[string]prov.ProvEntity)
	for _, ent := range entities {
		entityMap[ent.Info.ID] = ent
	}

	venueMap := make(map[string]prov.ProvVenue)
	for _, ven := range venues {
		venueMap[ven.Info.ID] = ven
	}

	return nodeMap, nodePathMap, entityMap, venueMap, nil
}

func (s *AssignmentService) ListAssignments(reqCtx prov.RequestContext, userID string) (*models.UserAssignmentsResponse, error) {
	roles, err := s.provClient.ListAllRoles(reqCtx)
	if err != nil {
		return nil, err
	}

	_, nodePathMap, _, _, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	var items []models.UserAssignment
	for _, r := range roles {
		userBound := false
		for _, u := range r.Users {
			if u == userID {
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
			if info, found := nodePathMap[scopeID]; found {
				scopeName = info.Name
				nodePath = info.Path
			} else {
				nodePath = []models.ScopePathItem{
					{
						ID:   scopeID,
						Type: scopeType,
						Name: scopeName,
					},
				}
			}

			items = append(items, models.UserAssignment{
				AssignmentID:       r.Info.ID,
				ScopeType:          scopeType,
				ScopeID:            scopeID,
				ScopeName:          scopeName,
				Role:               r.Info.Name,
				Path:               nodePath,
				ManagementRoleID:   r.Info.ID,
				ManagementPolicyID: r.ManagementPolicy,
				CreatedAt:          time.Unix(r.Info.Created, 0).UTC(),
			})
		}
	}

	// Avoid returning nil items
	if items == nil {
		items = []models.UserAssignment{}
	}

	return &models.UserAssignmentsResponse{
		Items: items,
	}, nil
}

func (s *AssignmentService) CreateAssignment(reqCtx prov.RequestContext, userID string, req *models.CreateUserAssignmentRequest) (*models.UserAssignment, bool, error) {
	// Validate the target user through OWSEC first
	if _, err := s.secClient.GetUser(reqCtx.Context, userID, reqCtx.BearerToken); err != nil {
		return nil, false, err
	}

	roles, err := s.provClient.ListAllRoles(reqCtx)
	if err != nil {
		return nil, false, err
	}

	var targetRole *prov.ProvManagementRole
	var isAlreadyAssigned bool

	for _, r := range roles {
		match := false
		if req.ScopeType == "entity" && r.Entity == req.ScopeID && r.Info.Name == req.Role {
			for _, u := range r.Users {
				if u == userID {
					match = true
					break
				}
			}
		} else if req.ScopeType == "venue" && r.Venue == req.ScopeID && r.Info.Name == req.Role {
			for _, u := range r.Users {
				if u == userID {
					match = true
					break
				}
			}
		}

		if match {
			targetRole = &r
			isAlreadyAssigned = true
			break
		}
	}

	if targetRole == nil {
		isAlreadyAssigned = false
		// 1. Resolve or create management policy
		var policyID string
		policies, err := s.provClient.ListAllPolicies(reqCtx)
		if err != nil {
			return nil, false, err
		}
		for _, p := range policies {
			if strings.EqualFold(p.Info.Name, req.Role) {
				policyID = p.Info.ID
				break
			}
		}

		if policyID == "" {
			callerInfo, err := s.secClient.ValidateToken(reqCtx.Context, reqCtx.BearerToken)
			if err != nil {
				return nil, false, fmt.Errorf("failed to validate caller token: %w", err)
			}
			callerRole := strings.ToLower(callerInfo.UserRole)
			if callerRole != "root" && callerRole != "system" {
				return nil, false, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("management policy for role %q does not exist; only root administrators can create system policies", req.Role))
			}

			newPolicyUUID := uuid.NewString()
			fixedEntries := getFixedPolicyEntries(req.Role)
			newPolicy := &prov.ProvManagementPolicy{
				Info: prov.ProvObjectInfo{
					Name:        req.Role,
					Description: "Fixed system policy for " + req.Role,
				},
				Entries: fixedEntries,
			}

			createdPolicy, err := s.provClient.CreatePolicy(reqCtx, newPolicyUUID, newPolicy)
			if err != nil {
				return nil, false, err
			}
			policyID = createdPolicy.Info.ID
		}

		// 2. Create management role specifically for this user
		newRoleUUID := uuid.NewString()
		newRole := &prov.ProvManagementRole{
			Info: prov.ProvObjectInfo{
				Name:        req.Role,
				Description: "Auto-generated role for " + req.Role + " assigned to " + userID,
			},
			ManagementPolicy: policyID,
			Users:            []string{userID},
		}
		if req.ScopeType == "entity" {
			newRole.Entity = req.ScopeID
		} else {
			newRole.Venue = req.ScopeID
		}

		createdRole, err := s.provClient.CreateRole(reqCtx, newRoleUUID, newRole)
		if err != nil {
			return nil, false, err
		}
		targetRole = createdRole
	}

	_, nodePathMap, _, _, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, false, err
	}

	scopeName := "Unknown Scope"
	var nodePath []models.ScopePathItem
	if info, found := nodePathMap[req.ScopeID]; found {
		scopeName = info.Name
		nodePath = info.Path
	} else {
		nodePath = []models.ScopePathItem{
			{
				ID:   req.ScopeID,
				Type: req.ScopeType,
				Name: scopeName,
			},
		}
	}

	return &models.UserAssignment{
		AssignmentID:       targetRole.Info.ID,
		ScopeType:          req.ScopeType,
		ScopeID:            req.ScopeID,
		ScopeName:          scopeName,
		Role:               targetRole.Info.Name,
		Path:               nodePath,
		ManagementRoleID:   targetRole.Info.ID,
		ManagementPolicyID: targetRole.ManagementPolicy,
		CreatedAt:          time.Unix(targetRole.Info.Created, 0).UTC(),
	}, isAlreadyAssigned, nil
}

func (s *AssignmentService) DeleteAssignment(reqCtx prov.RequestContext, userID string, assignmentID string) error {
	r, err := s.provClient.GetRole(reqCtx, assignmentID)
	if err != nil {
		return err
	}

	foundIdx := -1
	for i, u := range r.Users {
		if u == userID {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return apperror.New(apperror.CodeNotFound, "user assignment not found")
	}

	// Remove user from role
	r.Users = append(r.Users[:foundIdx], r.Users[foundIdx+1:]...)

	if len(r.Users) == 0 {
		// Delete role if no users left
		return s.provClient.DeleteRole(reqCtx, assignmentID)
	}

	_, err = s.provClient.UpdateRole(reqCtx, assignmentID, r)
	return err
}

func (s *AssignmentService) GetAccessPolicy(reqCtx prov.RequestContext, userID string, scope string, entityID string, venueID string, roleTemplate string) (*models.UserAccessPolicy, error) {
	if scope == "venue" && venueID != "" {
		_, _, _, venueMap, err := s.getLookupMaps(reqCtx)
		if err != nil {
			return nil, err
		}
		v, ok := venueMap[venueID]
		if !ok {
			return nil, apperror.New(apperror.CodeNotFound, "venue not found")
		}
		if v.Entity != entityID {
			return nil, apperror.New(apperror.CodeInvalidInput, "the specified venue does not belong to the specified entity")
		}
	}

	roles, err := s.provClient.ListAllRoles(reqCtx)
	if err != nil {
		return nil, err
	}

	var targetRole *prov.ProvManagementRole
	for _, r := range roles {
		userBound := false
		for _, u := range r.Users {
			if u == userID {
				userBound = true
				break
			}
		}

		if userBound {
			if roleTemplate != "" && r.Info.Name != roleTemplate {
				continue
			}

			if scope == "entity" && r.Entity == entityID {
				targetRole = &r
				break
			} else if scope == "venue" && r.Venue == venueID {
				targetRole = &r
				break
			}
		}
	}

	if targetRole == nil {
		return nil, apperror.New(apperror.CodeNotFound, "the access policy configuration for the target user at the specified scope does not exist")
	}

	policy, err := s.provClient.GetPolicy(reqCtx, targetRole.ManagementPolicy)
	if err != nil {
		return nil, err
	}

	var resourcePermissions []models.ResourcePermission
	for _, entry := range policy.Entries {
		for _, res := range entry.Resources {
			resourcePermissions = append(resourcePermissions, models.ResourcePermission{
				Resource: res,
				Policies: entry.Access,
			})
		}
	}

	resolvedEntityID := entityID

	return &models.UserAccessPolicy{
		Scope:               scope,
		EntityID:            resolvedEntityID,
		VenueID:             venueID,
		RoleTemplate:        targetRole.Info.Name,
		ResourcePermissions: resourcePermissions,
	}, nil
}

func getFixedPolicyEntries(role string) []prov.ProvManagementPolicyEntry {
	switch strings.ToLower(role) {
	case "root", "admin", "system":
		return []prov.ProvManagementPolicyEntry{
			{
				Access:    []string{"FULL"},
				Resources: []string{"entity", "venue", "device", "configuration", "managementPolicy", "managementRole"},
			},
		}
	case "csr":
		return []prov.ProvManagementPolicyEntry{
			{
				Access:    []string{"READ"},
				Resources: []string{"entity", "venue", "device", "configuration"},
			},
		}
	case "noc":
		return []prov.ProvManagementPolicyEntry{
			{
				Access:    []string{"READ"},
				Resources: []string{"entity", "venue"},
			},
			{
				Access:    []string{"READ", "MODIFY"},
				Resources: []string{"configuration", "device"},
			},
		}
	case "installer":
		return []prov.ProvManagementPolicyEntry{
			{
				Access:    []string{"READ"},
				Resources: []string{"venue", "configuration"},
			},
			{
				Access:    []string{"READ", "MODIFY"},
				Resources: []string{"device"},
			},
		}
	default:
		// Default fallback is read-only for safety
		return []prov.ProvManagementPolicyEntry{
			{
				Access:    []string{"READ"},
				Resources: []string{"entity", "venue", "device", "configuration", "managementPolicy", "managementRole"},
			},
		}
	}
}
