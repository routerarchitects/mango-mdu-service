package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type VenueService struct {
	provClient *prov.Client
}

func NewVenueService(provClient *prov.Client) *VenueService {
	return &VenueService{
		provClient: provClient,
	}
}

func (s *VenueService) getLookupMaps(reqCtx prov.RequestContext) (map[string]prov.ProvTreeNode, map[string]treeNodeInfo, map[string]prov.ProvEntity, map[string]prov.ProvVenue, map[string][]prov.ProvManagementRole, error) {
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

func (s *VenueService) GetVenue(reqCtx prov.RequestContext, uuidStr string) (*models.VenueDetail, error) {
	provVen, err := s.provClient.GetVenue(reqCtx, uuidStr)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToVenueDetail(provVen, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *VenueService) CreateVenue(reqCtx prov.RequestContext, req *models.CreateVenueRequest) (*models.VenueDetail, error) {
	newUUID := uuid.NewString()

	parentVenueID := ""
	if req.ParentVenueID != nil {
		parentVenueID = *req.ParentVenueID
	}

	provVen := &prov.ProvVenue{
		Info: prov.ProvObjectInfo{
			Name:        req.Name,
			Description: req.Description,
		},
		Parent: parentVenueID,
	}

	created, err := s.provClient.CreateVenue(reqCtx, newUUID, provVen)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToVenueDetail(created, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *VenueService) UpdateVenue(reqCtx prov.RequestContext, uuidStr string, req *models.UpdateVenueRequest) (*models.VenueDetail, error) {
	curr, err := s.provClient.GetVenue(reqCtx, uuidStr)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		curr.Info.Name = *req.Name
	}
	if req.Description != nil {
		curr.Info.Description = *req.Description
	}

	updated, err := s.provClient.UpdateVenue(reqCtx, uuidStr, curr)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToVenueDetail(updated, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}

func (s *VenueService) DeleteVenue(reqCtx prov.RequestContext, uuidStr string) error {
	return s.provClient.DeleteVenue(reqCtx, uuidStr)
}

func (s *VenueService) ListVenues(reqCtx prov.RequestContext, limit, offset int) (*models.VenueListResponse, error) {
	provList, err := s.provClient.ListVenues(reqCtx, limit, offset)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	var items []models.VenueSummary
	for _, ven := range provList {
		items = append(items, s.mapProvToVenueSummary(&ven, nodeMap, nodePathMap, entityMap, venueMap, roleMap))
	}

	return &models.VenueListResponse{
		Items: items,
		Metadata: models.ListMetadata{
			Total:  len(items),
			Limit:  limit,
			Offset: offset,
		},
	}, nil
}

func (s *VenueService) mapProvToVenueSummary(ven *prov.ProvVenue, nodeMap map[string]prov.ProvTreeNode, nodePathMap map[string]treeNodeInfo, entityMap map[string]prov.ProvEntity, venueMap map[string]prov.ProvVenue, roleMap map[string][]prov.ProvManagementRole) models.VenueSummary {
	var parentVenueID *string
	if ven.Parent != "" {
		parentVenueID = &ven.Parent
	}

	var path []models.ScopePathItem
	if info, found := nodePathMap[ven.Info.ID]; found {
		path = info.Path
	}

	// Compute recursive counts
	var deviceCount int
	if node, found := nodeMap[ven.Info.ID]; found {
		_, _, _, deviceCount = computeRecursiveCounts(node, entityMap, venueMap, roleMap)
	}

	return models.VenueSummary{
		ID:            ven.Info.ID,
		Name:          ven.Info.Name,
		EntityID:      ven.Entity,
		ParentVenueID: parentVenueID,
		Path:          path,
		DeviceCount:   deviceCount,
	}
}

func (s *VenueService) mapProvToVenueDetail(ven *prov.ProvVenue, nodeMap map[string]prov.ProvTreeNode, nodePathMap map[string]treeNodeInfo, entityMap map[string]prov.ProvEntity, venueMap map[string]prov.ProvVenue, roleMap map[string][]prov.ProvManagementRole) *models.VenueDetail {
	summary := s.mapProvToVenueSummary(ven, nodeMap, nodePathMap, entityMap, venueMap, roleMap)

	var managementRoleIDs []string
	for _, rList := range roleMap {
		for _, r := range rList {
			if r.Venue == ven.Info.ID {
				managementRoleIDs = append(managementRoleIDs, r.Info.ID)
			}
		}
	}

	createdAt := time.Unix(ven.Info.Created, 0).UTC()
	updatedAt := time.Unix(ven.Info.Modified, 0).UTC()

	return &models.VenueDetail{
		VenueSummary:       summary,
		Description:        ven.Info.Description,
		ManagementPolicyID: ven.ManagementPolicy,
		ManagementRoleIDs:  managementRoleIDs,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}

func (s *VenueService) ListEntityVenues(reqCtx prov.RequestContext, entityID string, limit, offset int) (*models.VenueListResponse, error) {
	provList, err := s.provClient.ListVenues(reqCtx, 1000, 0)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	var items []models.VenueSummary
	for _, ven := range provList {
		if ven.Entity == entityID {
			items = append(items, s.mapProvToVenueSummary(&ven, nodeMap, nodePathMap, entityMap, venueMap, roleMap))
		}
	}

	total := len(items)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginated := items[start:end]

	return &models.VenueListResponse{
		Items: paginated,
		Metadata: models.ListMetadata{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}, nil
}

func (s *VenueService) CreateEntityVenue(reqCtx prov.RequestContext, entityID string, req *models.CreateVenueRequest) (*models.VenueDetail, error) {
	newUUID := uuid.NewString()

	// A venue under an entity must not have a parent venue (owprov allows EITHER entity OR parent venue, not both)
	provVen := &prov.ProvVenue{
		Info: prov.ProvObjectInfo{
			Name:        req.Name,
			Description: req.Description,
		},
		Entity: entityID,
	}

	created, err := s.provClient.CreateVenue(reqCtx, newUUID, provVen)
	if err != nil {
		return nil, err
	}

	nodeMap, nodePathMap, entityMap, venueMap, roleMap, err := s.getLookupMaps(reqCtx)
	if err != nil {
		return nil, err
	}

	return s.mapProvToVenueDetail(created, nodeMap, nodePathMap, entityMap, venueMap, roleMap), nil
}
