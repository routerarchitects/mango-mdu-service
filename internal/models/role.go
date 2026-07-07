package models

import "time"

type ManagementRole struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	ManagementPolicy string    `json:"managementPolicy"`
	Users            []string  `json:"users"`
	Entity           string    `json:"entity,omitempty"`
	Venue            string    `json:"venue,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ManagementRoleListResponse struct {
	Items    []ManagementRole `json:"items"`
	Metadata ListMetadata     `json:"metadata"`
}

type CreateRoleRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	ManagementPolicy string   `json:"managementPolicy"`
	Users            []string `json:"users,omitempty"`
	Entity           string   `json:"entity,omitempty"`
	Venue            string   `json:"venue,omitempty"`
}

type UpdateRoleRequest struct {
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	ManagementPolicy *string   `json:"managementPolicy,omitempty"`
	Users            []string  `json:"users,omitempty"`
}
