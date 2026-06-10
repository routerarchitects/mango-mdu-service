package saga

import "time"

type ExecutionStatus string

const (
	ExecutionStatusPending                    ExecutionStatus = "pending"
	ExecutionStatusRunning                    ExecutionStatus = "running"
	ExecutionStatusCompleted                  ExecutionStatus = "completed"
	ExecutionStatusFailed                     ExecutionStatus = "failed"
	ExecutionStatusManualInterventionRequired ExecutionStatus = "manual_intervention_required"
)

type StepStatus string

const (
	StepStatusPending                    StepStatus = "pending"
	StepStatusCompleted                  StepStatus = "completed"
	StepStatusFailed                     StepStatus = "failed"
	StepStatusManualInterventionRequired StepStatus = "manual_intervention_required"
)

type CompensationStatus string

const (
	CompensationStatusPending                    CompensationStatus = "pending"
	CompensationStatusRunning                    CompensationStatus = "running"
	CompensationStatusSuccessful                 CompensationStatus = "successful"
	CompensationStatusFailed                     CompensationStatus = "failed"
	CompensationStatusManualInterventionRequired CompensationStatus = "manual_intervention_required"
)

type Execution struct {
	ID             string
	SagaType       string
	IdempotencyKey string
	Status         ExecutionStatus
	CurrentStep    string
	FailedStep     string
	RequestJSON    []byte
	ResponseJSON   []byte
	ErrorJSON      []byte
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Step struct {
	ID                   string
	SagaID               string
	StepNo               int
	StepName             string
	Status               StepStatus
	ResourceType         string
	ResourceID           string
	RequestJSON          []byte
	ResponseJSON         []byte
	ErrorJSON            []byte
	BeforeJSON           []byte
	AfterJSON            []byte
	CompensationRequired bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Compensation struct {
	ID                 string
	SagaID             string
	SagaStepID         string
	CompensationType   string
	CompensationStatus CompensationStatus
	ResourceType       string
	ResourceID         string
	RequestJSON        []byte
	ResponseJSON       []byte
	ErrorJSON          []byte
	ErrorMessage       string
	RetryCount         int
	MaxRetries         int
	NextRetryAt        *time.Time
	LastAttemptAt      *time.Time
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
