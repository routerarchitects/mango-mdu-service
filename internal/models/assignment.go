package models

import "time"

type UserAssignment struct {
	AssignmentID       string          `json:"assignmentId"`
	ScopeType          string          `json:"scopeType"` // entity, venue
	ScopeID            string          `json:"scopeId"`
	ScopeName          string          `json:"scopeName"`
	Role               string          `json:"role"` // RoleKey
	Path               []ScopePathItem `json:"path"`
	ManagementRoleID   string          `json:"managementRoleId,omitempty"`
	ManagementPolicyID string          `json:"managementPolicyId,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type UserAssignmentsResponse struct {
	Items []UserAssignment `json:"items"`
}

type CreateUserAssignmentRequest struct {
	ScopeType string `json:"scopeType"` // entity, venue
	ScopeID   string `json:"scopeId"`
	Role      string `json:"role"` // RoleKey
}

type ResourcePermission struct {
	Resource string   `json:"resource"` // entity, venue, operator, inventory, configuration, management policy, management role
	Policies []string `json:"policies"` // PolicyAccessKey: NOACCESS, READ, MODIFY, DELETE, LIST, CREATE, FULL
}

type UserAccessPolicy struct {
	Scope               string               `json:"scope"` // entity, venue
	EntityID            string               `json:"entityId"`
	VenueID             string               `json:"venueId,omitempty"`
	RoleTemplate        string               `json:"roleTemplate"`
	ResourcePermissions []ResourcePermission `json:"resourcePermissions"`
}
