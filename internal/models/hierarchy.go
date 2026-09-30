package models

type ScopePathItem struct {
	ID   string `json:"id"`
	Type string `json:"type"` // entity, venue
	Name string `json:"name"`
}

type HierarchyNodeSummary struct {
	EntityCount int `json:"entityCount"`
	VenueCount  int `json:"venueCount"`
	UserCount   int `json:"userCount"`
	DeviceCount int `json:"deviceCount"`
}

type HierarchyNode struct {
	ID          string                `json:"id"`
	Type        string                `json:"type"` // entity, venue
	Name        string                `json:"name"`
	ParentID    *string               `json:"parentId"` // nullable
	Path        []ScopePathItem       `json:"path"`
	Selectable  bool                  `json:"selectable"`
	HasChildren bool                  `json:"hasChildren"`
	Children    []HierarchyNode       `json:"children,omitempty"`
	Summary     *HierarchyNodeSummary `json:"summary,omitempty"`
}

type HierarchyTreeResponse struct {
	Roots []HierarchyNode `json:"roots"`
}
