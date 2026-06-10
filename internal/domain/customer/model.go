package customer

import "time"

type Status string

const (
	StatusActive       Status = "active"
	StatusDeleting     Status = "deleting"
	StatusDeleteFailed Status = "delete_failed"
	StatusDeleted      Status = "deleted"
)

type Customer struct {
	ID              string
	EntityID        string
	CreatedByUserID string
	CreatedByEmail  string
	Name            string
	PhoneNumber     string
	LocationJSON    []byte
	Status          Status
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
