package models

import "time"

type ManagementPolicyEntry struct {
	Users     []string `json:"users,omitempty"`
	Resources []string `json:"resources"`
	Access    []string `json:"access"` // READ, MODIFY, DELETE, NOACCESS
}

type ManagementPolicy struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Entries     []ManagementPolicyEntry `json:"entries"`
	Entity      string                  `json:"entity,omitempty"`
	Venue       string                  `json:"venue,omitempty"`
	CreatedAt   time.Time               `json:"createdAt"`
	UpdatedAt   time.Time               `json:"updatedAt"`
}

type ManagementPolicyListResponse struct {
	Items    []ManagementPolicy `json:"items"`
	Metadata ListMetadata       `json:"metadata"`
}

type CreatePolicyRequest struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Entries     []ManagementPolicyEntry `json:"entries"`
	Entity      string                  `json:"entity,omitempty"`
	Venue       string                  `json:"venue,omitempty"`
}

type UpdatePolicyRequest struct {
	Name        *string                 `json:"name,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Entries     []ManagementPolicyEntry `json:"entries,omitempty"`
}
