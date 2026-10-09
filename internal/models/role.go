package models

// ManagementRole represents a management role assignment record in OWPROV.
type ManagementRole struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	ManagementPolicy string   `json:"managementPolicy"`
	Users            []string `json:"users"`
	Entity           string   `json:"entity,omitempty"`
	Venue            string   `json:"venue,omitempty"`
	Created          int64    `json:"created,omitempty"`
	Modified         int64    `json:"modified,omitempty"`
}

// ManagementRoleListResponse represents the JSON response envelope from OWPROV for role queries.
type ManagementRoleListResponse struct {
	Roles []ManagementRole `json:"roles"`
}
