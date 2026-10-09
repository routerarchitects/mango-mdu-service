package models

// Entity represents a property/entity record in OWPROV.
type Entity struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Venues      []string `json:"venues,omitempty"`
}

// EntityListResponse represents the JSON response envelope from OWPROV for entity queries.
type EntityListResponse struct {
	Entities []Entity `json:"entities"`
}
