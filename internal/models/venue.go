package models

// Venue represents a venue record in OWPROV.
type Venue struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Entity      string   `json:"entity,omitempty"`
	Boards      []string `json:"boards,omitempty"`
}

// VenueDetails represents the full venue record from OWPROV with boards and references.
type VenueDetails struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Entity      string   `json:"entity,omitempty"`
	Boards      []string `json:"boards,omitempty"`
	Devices     []string `json:"devices,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Children    []string `json:"children,omitempty"`
}

// VenueListResponse represents the JSON response envelope from OWPROV for venue queries.
type VenueListResponse struct {
	Venues []Venue `json:"venues"`
}
