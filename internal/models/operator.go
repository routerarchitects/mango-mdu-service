package models

import "time"

type OperatorDetail struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	EntityID       string    `json:"entityId,omitempty"`
	RegistrationID string    `json:"registrationId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type UpdateOperatorRequest struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	RegistrationID *string `json:"registrationId,omitempty"`
}

type ProvOperator struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Created        int64  `json:"created"`
	Modified       int64  `json:"modified"`
	RegistrationID string `json:"registrationId"`
	EntityID       string `json:"entityId"`
}
