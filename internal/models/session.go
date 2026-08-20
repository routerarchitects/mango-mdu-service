package models

import "time"

type UserSummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name,omitempty"`
	Email       string     `json:"email,omitempty"`
	Role        string     `json:"role,omitempty"`
	Status      string     `json:"status,omitempty"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

type RbacDecision struct {
	Allowed bool   `json:"allowed"`
	Mode    string `json:"mode"` // hidden, read_only, interactive
	Reason  string `json:"reason,omitempty"`
}

type EffectivePermissionSet struct {
	Hierarchy      RbacDecision `json:"hierarchy"`
	Users          RbacDecision `json:"users"`
	Billing        RbacDecision `json:"billing"`
	Configurations RbacDecision `json:"configurations"`
	Devices        RbacDecision `json:"devices"`
}

type SessionAssignment struct {
	AssignmentID       string          `json:"assignmentId"`
	ScopeType          string          `json:"scopeType"` // entity, venue
	ScopeID            string          `json:"scopeId"`
	ScopeName          string          `json:"scopeName"`
	Path               []ScopePathItem `json:"path"`
	Role               string          `json:"role"` // RoleKey
	ManagementRoleID   string          `json:"managementRoleId,omitempty"`
	ManagementPolicyID string          `json:"managementPolicyId,omitempty"`
}

type SessionContext struct {
	User        UserSummary            `json:"user"`
	ActiveScope *ScopePathItem         `json:"activeScope,omitempty"`
	Assignments []SessionAssignment    `json:"assignments"`
	Permissions EffectivePermissionSet `json:"permissions"`
}
