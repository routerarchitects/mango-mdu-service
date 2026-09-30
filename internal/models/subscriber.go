package models

import "time"

type SubscriberSignup struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	UserID         string    `json:"userId,omitempty"`
	OperatorID     string    `json:"operatorId,omitempty"`
	MacAddress     string    `json:"macAddress,omitempty"`
	SerialNumber   string    `json:"serialNumber,omitempty"`
	Status         string    `json:"status"`
	RegistrationID string    `json:"registrationId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SubscriberListResponse struct {
	Items []SubscriberSignup `json:"items"`
}
