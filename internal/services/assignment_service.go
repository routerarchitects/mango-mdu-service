package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type AssignmentService struct {
	provClient *prov.Client
}

func NewAssignmentService(provClient *prov.Client) *AssignmentService {
	return &AssignmentService{
		provClient: provClient,
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

	entities, err := s.provClient.ListEntities(reqCtx, 1000, 0)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	venues, err := s.provClient.ListVenues(reqCtx, 1000, 0)
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
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
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
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
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
		// 1. Resolve or create management policy specifically for this user
		var policyID string
		policies, err := s.provClient.ListPolicies(reqCtx, 1000, 0)
		if err == nil {
			for _, p := range policies {
				match := false
				policyName := req.Role + "Policy-" + userID
				if req.ScopeType == "entity" && p.Entity == req.ScopeID && p.Info.Name == policyName {
					match = true
				} else if req.ScopeType == "venue" && p.Venue == req.ScopeID && p.Info.Name == policyName {
					match = true
				}

				if match {
					policyID = p.Info.ID
					break
				}
			}
		}

		if policyID == "" {
			newPolicyUUID := uuid.NewString()
			defaultEntries := getDefaultPolicyEntries(req.Role, userID, req.ScopeType, req.ScopeID)

			newPolicy := &prov.ProvManagementPolicy{
				Info: prov.ProvObjectInfo{
					Name:        req.Role + "Policy-" + userID,
					Description: "Auto-generated policy for " + req.Role + " assigned to " + userID,
				},
				Entries: defaultEntries,
			}
			if req.ScopeType == "entity" {
				newPolicy.Entity = req.ScopeID
			} else {
				newPolicy.Venue = req.ScopeID
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
		err = s.provClient.DeleteRole(reqCtx, assignmentID)
		if err != nil {
			return err
		}
		// Also delete the associated policy
		return s.provClient.DeletePolicy(reqCtx, r.ManagementPolicy)
	}

	_, err = s.provClient.UpdateRole(reqCtx, assignmentID, r)
	return err
}

func (s *AssignmentService) GetAccessPolicy(reqCtx prov.RequestContext, userID string, scope string, entityID string, venueID string) (*models.UserAccessPolicy, error) {
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
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
	if scope == "venue" && venueID != "" {
		_, _, _, venueMap, err := s.getLookupMaps(reqCtx)
		if err == nil {
			if v, ok := venueMap[venueID]; ok && v.Entity != "" {
				resolvedEntityID = v.Entity
			}
		}
	}

	return &models.UserAccessPolicy{
		Scope:               scope,
		EntityID:            resolvedEntityID,
		VenueID:             venueID,
		RoleTemplate:        targetRole.Info.Name,
		ResourcePermissions: resourcePermissions,
	}, nil
}

func (s *AssignmentService) UpdateAccessPolicy(reqCtx prov.RequestContext, userID string, policy *models.UserAccessPolicy) (*models.UserAccessPolicy, error) {
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
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
			if policy.Scope == "entity" && r.Entity == policy.EntityID {
				targetRole = &r
				break
			} else if policy.Scope == "venue" && r.Venue == policy.VenueID {
				targetRole = &r
				break
			}
		}
	}

	if targetRole == nil {
		return nil, apperror.New(apperror.CodeNotFound, "user assignment not found")
	}

	// Group ResourcePermissions into ManagementPolicyEntries
	accessMap := make(map[string][]string) // key: joined access list, val: resources list
	var accessKeys []string

	for _, rp := range policy.ResourcePermissions {
		// Sort or format access list to make a deterministic key
		joinedAccess := ""
		for i, a := range rp.Policies {
			if i > 0 {
				joinedAccess += ","
			}
			joinedAccess += a
		}
		if _, exists := accessMap[joinedAccess]; !exists {
			accessKeys = append(accessKeys, joinedAccess)
		}
		accessMap[joinedAccess] = append(accessMap[joinedAccess], rp.Resource)
	}

	var entries []prov.ProvManagementPolicyEntry
	for _, k := range accessKeys {
		var accessList []string
		if k != "" {
			accessList = stringsSplit(k, ",")
		}
		var policyJSON string
		if policy.Scope == "entity" {
			policyJSON = fmt.Sprintf(`{"type":"entity","entityId":"%s","includeVenues":true,"includeChildEntities":true}`, policy.EntityID)
		} else if policy.Scope == "venue" {
			policyJSON = fmt.Sprintf(`{"type":"venue","venueId":"%s","includeVenues":true,"includeChildEntities":true}`, policy.VenueID)
		}
		entries = append(entries, prov.ProvManagementPolicyEntry{
			Users:     []string{userID},
			Resources: accessMap[k],
			Access:    accessList,
			Policy:    policyJSON,
		})
	}

	provPolicy, err := s.provClient.GetPolicy(reqCtx, targetRole.ManagementPolicy)
	if err != nil {
		return nil, err
	}

	provPolicy.Entries = entries
	_, err = s.provClient.UpdatePolicy(reqCtx, provPolicy.Info.ID, provPolicy)
	if err != nil {
		return nil, err
	}

	resolvedEntityID := policy.EntityID
	if policy.Scope == "venue" && policy.VenueID != "" {
		_, _, _, venueMap, err := s.getLookupMaps(reqCtx)
		if err == nil {
			if v, ok := venueMap[policy.VenueID]; ok && v.Entity != "" {
				resolvedEntityID = v.Entity
			}
		}
	}

	return &models.UserAccessPolicy{
		Scope:               policy.Scope,
		EntityID:            resolvedEntityID,
		VenueID:             policy.VenueID,
		RoleTemplate:        targetRole.Info.Name,
		ResourcePermissions: policy.ResourcePermissions,
	}, nil
}

func stringsSplit(s, sep string) []string {
	var res []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			res = append(res, s[start:i])
			start = i + 1
		}
	}
	res = append(res, s[start:])
	return res
}

func getDefaultPolicyEntries(role string, userID string, scopeType string, scopeID string) []prov.ProvManagementPolicyEntry {
	var policyJSON string
	if scopeType == "entity" {
		policyJSON = fmt.Sprintf(`{"type":"entity","entityId":"%s","includeVenues":true,"includeChildEntities":true}`, scopeID)
	} else if scopeType == "venue" {
		policyJSON = fmt.Sprintf(`{"type":"venue","venueId":"%s","includeVenues":true,"includeChildEntities":true}`, scopeID)
	}

	allResources := []string{"entity", "venue", "operator", "inventory", "configuration", "managementPolicy", "managementRole"}

	switch role {
	case "root", "admin", "system":
		// Full access to all resources
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: allResources,
				Access:    []string{"READ", "LIST", "CREATE", "UPDATE", "MODIFY", "DELETE"},
				Policy:    policyJSON,
			},
		}
	case "csr":
		// Read-only to all resources
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: allResources,
				Access:    []string{"READ", "LIST"},
				Policy:    policyJSON,
			},
		}
	case "installer":
		// Read/Write configuration and inventory
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: []string{"configuration", "inventory"},
				Access:    []string{"READ", "LIST", "UPDATE", "MODIFY"},
				Policy:    policyJSON,
			},
		}
	case "noc":
		// Read-only for structural, Read/Write for configuration and inventory
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: []string{"entity", "venue", "operator", "managementPolicy", "managementRole"},
				Access:    []string{"READ", "LIST"},
				Policy:    policyJSON,
			},
			{
				Users:     []string{userID},
				Resources: []string{"configuration", "inventory"},
				Access:    []string{"READ", "LIST", "UPDATE", "MODIFY"},
				Policy:    policyJSON,
			},
		}
	case "accounting":
		// Read-only to structural resources
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: []string{"entity", "venue", "operator", "managementPolicy", "managementRole"},
				Access:    []string{"READ", "LIST"},
				Policy:    policyJSON,
			},
		}
	default:
		// Default fallback is read-only for safety
		return []prov.ProvManagementPolicyEntry{
			{
				Users:     []string{userID},
				Resources: allResources,
				Access:    []string{"READ", "LIST"},
				Policy:    policyJSON,
			},
		}
	}
}
