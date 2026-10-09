package models

// Venue represents a venue record in OWPROV.
type Venue struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Entity      string `json:"entity,omitempty"`
}

// VenueListResponse represents the JSON response envelope from OWPROV for venue queries.
type VenueListResponse struct {
	Venues []Venue `json:"venues"`
}
