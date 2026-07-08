package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type EntityService struct {
	provClient *prov.Client
}

func NewEntityService(provClient *prov.Client) *EntityService {
	return &EntityService{
		provClient: provClient,
	}
}

func (s *EntityService) getLookupMaps(reqCtx prov.RequestContext) (map[string]prov.ProvTreeNode, map[string]treeNodeInfo, map[string]prov.ProvEntity, map[string]prov.ProvVenue, map[string][]prov.ProvManagementRole, error) {
	tree, err := s.provClient.GetTree(reqCtx)
	if err != nil {
		return nil, nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, nil, err
	}
	venues, err := s.provClient.ListVenues(reqCtx, 1000, 0)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	roles, err := s.provClient.ListRoles(reqCtx, 1000, 0)
	if err != nil {
		return nil, nil, nil, nil, nil, err
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

	return nodeMap, nodePathMap, entityMap, venueMap, roleMap, nil
}

func (s *EntityService) GetEntity(reqCtx prov.RequestContext, uuidStr string) (*models.EntityDetail, error) {
	provEnt, err := s.provClient.GetEntity(reqCtx, uuidStr)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToEntityDetail(provEnt, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *EntityService) CreateEntity(reqCtx prov.RequestContext, req *models.CreateEntityRequest) (*models.EntityDetail, error) {
	newUUID := uuid.NewString()

	parentID := ""
	if req.ParentEntityID != nil {
		parentID = *req.ParentEntityID
	}

	entType := "normal"
	if req.Type != "" {
		entType = req.Type
	}

	provEnt := &prov.ProvEntity{
		Info: prov.ProvObjectInfo{
			Name:        req.Name,
			Description: req.Description,
		},
		Parent: parentID,
		Type:   entType,
	}

	created, err := s.provClient.CreateEntity(reqCtx, newUUID, provEnt)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToEntityDetail(created, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *EntityService) UpdateEntity(reqCtx prov.RequestContext, uuidStr string, req *models.UpdateEntityRequest) (*models.EntityDetail, error) {
	curr, err := s.provClient.GetEntity(reqCtx, uuidStr)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		curr.Info.Name = *req.Name
	}
	if req.Description != nil {
		curr.Info.Description = *req.Description
	}

	updated, err := s.provClient.UpdateEntity(reqCtx, uuidStr, curr)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToEntityDetail(updated, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *EntityService) DeleteEntity(reqCtx prov.RequestContext, uuidStr string) error {
	return s.provClient.DeleteEntity(reqCtx, uuidStr)
}

func (s *EntityService) ListEntities(reqCtx prov.RequestContext, limit, offset int) (*models.EntityListResponse, error) {
	provList, err := s.provClient.ListEntities(reqCtx, limit, offset)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	var items []models.EntitySummary
	for _, ent := range provList {
		items = append(items, s.mapProvToEntitySummary(&ent, nodeMap, nodePathMap, entityMap, venueMap, roleMap))
	}

	return &models.EntityListResponse{
		Items: items,
		Metadata: models.ListMetadata{
			Total:  len(items), // In a real backend this would be database total, but since we map stateless list we can use len
			Limit:  limit,
			Offset: offset,
		},
	}, nil
}

func (s *EntityService) mapProvToEntitySummary(ent *prov.ProvEntity, nodeMap map[string]prov.ProvTreeNode, nodePathMap map[string]treeNodeInfo, entityMap map[string]prov.ProvEntity, venueMap map[string]prov.ProvVenue, roleMap map[string][]prov.ProvManagementRole) models.EntitySummary {
	var parentID *string
	if ent.Parent != "" {
		parentID = &ent.Parent
	}

	var path []models.ScopePathItem
	if info, found := nodePathMap[ent.Info.ID]; found {
		path = info.Path
	}

	// Compute recursive counts
	var venueCount, userCount, deviceCount int
	if node, found := nodeMap[ent.Info.ID]; found {
		_, venueCount, userCount, deviceCount = computeRecursiveCounts(node, entityMap, venueMap, roleMap)
	}

	entType := ent.Type
	if entType == "" {
		entType = "normal"
	}

	return models.EntitySummary{
		ID:          ent.Info.ID,
		Name:        ent.Info.Name,
		ParentID:    parentID,
		Type:        entType,
		Path:        path,
		VenueCount:  venueCount,
		UserCount:   userCount,
		DeviceCount: deviceCount,
	}
}

func (s *EntityService) mapProvToEntityDetail(ent *prov.ProvEntity, nodeMap map[string]prov.ProvTreeNode, nodePathMap map[string]treeNodeInfo, entityMap map[string]prov.ProvEntity, venueMap map[string]prov.ProvVenue, roleMap map[string][]prov.ProvManagementRole) *models.EntityDetail {
	summary := s.mapProvToEntitySummary(ent, nodeMap, nodePathMap, entityMap, venueMap, roleMap)

	// In mango-mdu, managementRoleIds are roles associated with this entity.
	// We scan roles for matches.
	var managementRoleIDs []string
	for _, rList := range roleMap {
		for _, r := range rList {
			if r.Entity == ent.Info.ID {
				managementRoleIDs = append(managementRoleIDs, r.Info.ID)
			}
		}
	}

	createdAt := time.Unix(ent.Info.Created, 0).UTC()
	updatedAt := time.Unix(ent.Info.Modified, 0).UTC()

	return &models.EntityDetail{
		EntitySummary:      summary,
		Description:        ent.Info.Description,
		OperatorID:         ent.OperatorID,
		ManagementPolicyID: ent.ManagementPolicy,
		ManagementRoleIDs:  managementRoleIDs,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}

func computeRecursiveCounts(node prov.ProvTreeNode, entityMap map[string]prov.ProvEntity, venueMap map[string]prov.ProvVenue, roleMap map[string][]prov.ProvManagementRole) (entityCount, venueCount, userCount, deviceCount int) {
	deviceSet := make(map[string]bool)
	userSet := make(map[string]bool)
	var selfEntities int
	var selfVenues int
	if node.Type == "entity" {
		if node.UUID != "00000000-0000-0000-0000-000000000000" {
			selfEntities = 1
		}
	} else if node.Type == "venue" {
		selfVenues = 1
	}

	var collect func(n prov.ProvTreeNode)
	collect = func(n prov.ProvTreeNode) {
		if n.Type == "entity" {
			if ent, ok := entityMap[n.UUID]; ok {
				for _, d := range ent.Devices {
					deviceSet[d] = true
				}
			}
		} else if n.Type == "venue" {
			if ven, ok := venueMap[n.UUID]; ok {
				for _, d := range ven.Devices {
					deviceSet[d] = true
				}
			}
		}
		for _, r := range roleMap[n.UUID] {
			for _, u := range r.Users {
				userSet[u] = true
			}
		}
		for _, c := range n.Children {
			if c.Type == "entity" {
				selfEntities++
			} else if c.Type == "venue" {
				selfVenues++
			}
			collect(c)
		}
		for _, v := range n.Venues {
			selfVenues++
			collect(v)
		}
	}

	collect(node)
	return selfEntities, selfVenues, len(userSet), len(deviceSet)
}
