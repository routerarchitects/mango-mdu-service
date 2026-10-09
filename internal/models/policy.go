package models

// PolicyMetadata represents management policy details returned in PolicyOverviewResponse.
type PolicyMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Entity      string `json:"entity"`
	Venue       string `json:"venue"`
	Created     int64  `json:"created"`
	Modified    int64  `json:"modified"`
}

// UserPolicyScopeInfo represents a single scoped property/venue role assignment for a user.
type UserPolicyScopeInfo struct {
	EntityID   string `json:"entityId"`
	EntityName string `json:"entityName"`
	VenueID    string `json:"venueId"`
	VenueName  string `json:"venueName"`
}

// UserWithPolicySummary represents an itemized unique user assigned to a policy.
type UserWithPolicySummary struct {
	ID                     string                `json:"id"`
	Name                   string                `json:"name"`
	Email                  string                `json:"email"`
	UserRole               string                `json:"userRole"`
	Avatar                 string                `json:"avatar,omitempty"`
	ScopedAssignmentsCount int                   `json:"scopedAssignmentsCount"`
	Scopes                 []UserPolicyScopeInfo `json:"scopes"`
}

// PolicyOverviewResponse represents the consolidated northbound response for GET /api/v1/policy/{id}/overview.
type PolicyOverviewResponse struct {
	Policy                 PolicyMetadata          `json:"policy"`
	TotalUsers             int                     `json:"totalUsers"`
	TotalScopedAssignments int                     `json:"totalScopedAssignments"`
	TotalProperties        int                     `json:"totalProperties"`
	TotalVenues            int                     `json:"totalVenues"`
	UsersWithPolicy        []UserWithPolicySummary `json:"usersWithPolicy"`
}

// ManagementPolicyEntry represents a single resource access permission in OWPROV.
type ManagementPolicyEntry struct {
	Resources []string `json:"resources"`
	Access    []string `json:"access"`
}

// ManagementPolicy represents a policy record in OWPROV.
type ManagementPolicy struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Entity      string                  `json:"entity,omitempty"`
	Venue       string                  `json:"venue,omitempty"`
	Entries     []ManagementPolicyEntry `json:"entries,omitempty"`
	Created     int64                   `json:"created,omitempty"`
	Modified    int64                   `json:"modified,omitempty"`
}
