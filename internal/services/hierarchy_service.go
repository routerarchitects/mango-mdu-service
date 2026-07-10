package services

import (
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type HierarchyService struct {
	provClient *prov.Client
}

func NewHierarchyService(provClient *prov.Client) *HierarchyService {
	return &HierarchyService{
		provClient: provClient,
	}
}

type hierarchyBuilder struct {
	entityMap map[string]prov.ProvEntity
	venueMap  map[string]prov.ProvVenue
	roleMap   map[string][]prov.ProvManagementRole // scopeID -> roles
}

func (b *hierarchyBuilder) build(node prov.ProvTreeNode, parentID *string, parentPath []models.ScopePathItem) models.HierarchyNode {
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

	// Build children recursively
	var children []models.HierarchyNode
	for _, child := range node.Children {
		children = append(children, b.build(child, &node.UUID, nodePath))
	}
	for _, venue := range node.Venues {
		children = append(children, b.build(venue, &node.UUID, nodePath))
	}

	// Self counts
	var selfEntities int
	var selfVenues int
	if node.Type == "entity" {
		if node.UUID != "00000000-0000-0000-0000-000000000000" {
			selfEntities = 1
		}
	} else if node.Type == "venue" {
		selfVenues = 1
	}

	deviceSet := make(map[string]bool)
	userSet := make(map[string]bool)

	// Add self devices
	if node.Type == "entity" {
		if ent, ok := b.entityMap[node.UUID]; ok {
			for _, d := range ent.Devices {
				deviceSet[d] = true
			}
		}
	} else if node.Type == "venue" {
		if ven, ok := b.venueMap[node.UUID]; ok {
			for _, d := range ven.Devices {
				deviceSet[d] = true
			}
		}
	}

	// Add self users
	for _, r := range b.roleMap[node.UUID] {
		for _, u := range r.Users {
			userSet[u] = true
		}
	}

	// Aggregate from children
	var childEntities int
	var childVenues int
	for _, childNode := range children {
		childEntities += childNode.Summary.EntityCount
		childVenues += childNode.Summary.VenueCount
	}

	// Recursively collect unique devices and users from descendants
	var collectDescendants func(n prov.ProvTreeNode)
	collectDescendants = func(n prov.ProvTreeNode) {
		if n.Type == "entity" {
			if ent, ok := b.entityMap[n.UUID]; ok {
				for _, d := range ent.Devices {
					deviceSet[d] = true
				}
			}
		} else if n.Type == "venue" {
			if ven, ok := b.venueMap[n.UUID]; ok {
				for _, d := range ven.Devices {
					deviceSet[d] = true
				}
			}
		}
		for _, r := range b.roleMap[n.UUID] {
			for _, u := range r.Users {
				userSet[u] = true
			}
		}
		for _, c := range n.Children {
			collectDescendants(c)
		}
		for _, v := range n.Venues {
			collectDescendants(v)
		}
	}

	for _, child := range node.Children {
		collectDescendants(child)
	}
	for _, venue := range node.Venues {
		collectDescendants(venue)
	}

	summary := &models.HierarchyNodeSummary{
		EntityCount: selfEntities + childEntities,
		VenueCount:  selfVenues + childVenues,
		UserCount:   len(userSet),
		DeviceCount: len(deviceSet),
	}

	hasChildren := len(children) > 0
	selectable := node.UUID != "00000000-0000-0000-0000-000000000000"

	return models.HierarchyNode{
		ID:          node.UUID,
		Type:        node.Type,
		Name:        node.Name,
		ParentID:    parentID,
		Path:        nodePath,
		Selectable:  selectable,
		HasChildren: hasChildren,
		Children:    children,
		Summary:     summary,
	}
}

func findNodeByID(nodes []models.HierarchyNode, targetID string) (models.HierarchyNode, bool) {
	for _, n := range nodes {
		if n.ID == targetID {
			return n, true
		}
		if found, ok := findNodeByID(n.Children, targetID); ok {
			return found, true
		}
	}
	return models.HierarchyNode{}, false
}

func (s *HierarchyService) GetHierarchyTree(reqCtx prov.RequestContext, scopeEntityID string) (*models.HierarchyTreeResponse, error) {
	// 1. Fetch the visible tree structure
	tree, err := s.provClient.GetTree(reqCtx)
	if err != nil {
		return nil, err
	}

	// 2. Load all entities, venues, and roles to resolve summaries
	entities, err := s.provClient.ListAllEntities(reqCtx)
	if err != nil {
		return nil, err
	}
	venues, err := s.provClient.ListAllVenues(reqCtx)
	if err != nil {
		return nil, err
	}
	roles, err := s.provClient.ListAllRoles(reqCtx)
	if err != nil {
		return nil, err
	}

	entityMap := make(map[string]prov.ProvEntity)
	for _, ent := range entities {
		entityMap[ent.Info.ID] = ent
	}

	venueMap := make(map[string]prov.ProvVenue)
	for _, ven := range venues {
		venueMap[ven.Info.ID] = ven
	}

	roleMap := make(map[string][]prov.ProvManagementRole)
	for _, r := range roles {
		scopeID := r.Entity
		if scopeID == "" {
			scopeID = r.Venue
		}
		if scopeID != "" {
			roleMap[scopeID] = append(roleMap[scopeID], r)
		}
	}

	builder := &hierarchyBuilder{
		entityMap: entityMap,
		venueMap:  venueMap,
		roleMap:   roleMap,
	}

	// 3. Build tree
	var roots []models.HierarchyNode
	if tree != nil {
		// If the tree returned is the dummy root with zero UUID and user has no access to the zero root,
		// we promote its children to roots.
		if tree.UUID == "00000000-0000-0000-0000-000000000000" {
			for _, child := range tree.Children {
				roots = append(roots, builder.build(child, nil, nil))
			}
			for _, venue := range tree.Venues {
				roots = append(roots, builder.build(venue, nil, nil))
			}
			// If roots list is empty, keep the dummy root
			if len(roots) == 0 {
				roots = append(roots, builder.build(*tree, nil, nil))
			}
		} else {
			roots = append(roots, builder.build(*tree, nil, nil))
		}
	}

	if scopeEntityID != "" {
		if found, ok := findNodeByID(roots, scopeEntityID); ok {
			// Clear parent ID for the new root of the shaped tree to represent it as the top level
			found.ParentID = nil
			roots = []models.HierarchyNode{found}
		} else {
			roots = []models.HierarchyNode{}
		}
	}

	return &models.HierarchyTreeResponse{
		Roots: roots,
	}, nil
}
