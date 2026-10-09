package models

// SecUser represents a user profile record in OWSEC.
type SecUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	UserRole    string `json:"userRole"`
	Avatar      string `json:"avatar,omitempty"`
	Description string `json:"description,omitempty"`
}

// SecUserListResponse represents the JSON response envelope from OWSEC for user queries.
type SecUserListResponse struct {
	Users []SecUser `json:"users"`
}
