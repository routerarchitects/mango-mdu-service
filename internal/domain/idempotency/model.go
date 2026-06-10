package idempotency

import "time"

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Key struct {
	Key          string
	RequestHash  string
	Status       Status
	ResponseJSON []byte
	ErrorJSON    []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
