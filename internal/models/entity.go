package models

import "time"

type EntitySummary struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	ParentID    *string         `json:"parentId"` // nullable
	Type        string          `json:"type"`     // normal, subscriber
	Path        []ScopePathItem `json:"path"`
	VenueCount  int             `json:"venueCount"`
	UserCount   int             `json:"userCount"`
	DeviceCount int             `json:"deviceCount"`
}

type EntityDetail struct {
	EntitySummary
	Description        string    `json:"description,omitempty"`
	OperatorID         string    `json:"operatorId,omitempty"`
	ManagementPolicyID string    `json:"managementPolicyId,omitempty"`
	ManagementRoleIDs  []string  `json:"managementRoleIds"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type CreateEntityRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	ParentEntityID *string `json:"parentEntityId,omitempty"`
	Type           string  `json:"type,omitempty"` // normal, subscriber
}

type UpdateEntityRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type EntityListResponse struct {
	Items    []EntitySummary `json:"items"`
	Metadata ListMetadata    `json:"metadata"`
}
