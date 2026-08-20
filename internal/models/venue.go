package models

import "time"

type VenueSummary struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	EntityID      string          `json:"entityId"`
	ParentVenueID *string         `json:"parentVenueId"` // nullable
	Path          []ScopePathItem `json:"path"`
	DeviceCount   int             `json:"deviceCount"`
}

type VenueDetail struct {
	VenueSummary
	Description        string    `json:"description,omitempty"`
	ManagementPolicyID string    `json:"managementPolicyId,omitempty"`
	ManagementRoleIDs  []string  `json:"managementRoleIds"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type CreateVenueRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	ParentVenueID *string `json:"parentVenueId,omitempty"`
}

type UpdateVenueRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type VenueListResponse struct {
	Items    []VenueSummary `json:"items"`
	Metadata ListMetadata   `json:"metadata"`
}
