package models

import "time"

// Workflow is a named, ordered pipeline documenting how one real request
// actually flows through the app today — trigger, hook, actions, services —
// the "workflow builder" view from payment-gateway.md, scoped to
// visualizing/configuring existing wiring rather than a full drag
// node-graph engine.
type Workflow struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex"`
	Slug        string `gorm:"uniqueIndex"`
	Description string
	Status      uint8 // 1 = active

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Recognized WorkflowStep.StepType values.
const (
	WorkflowStepTrigger = "trigger"
	WorkflowStepHook    = "hook"
	WorkflowStepAction  = "action"
	WorkflowStepService = "service"
)

// WorkflowStep is one node in a Workflow's pipeline, ordered by StepOrder.
// ConfigJSON is a free-form JSON object of the step's configured details
// (endpoint, hook key, gateway fields, etc.), rendered as a key/value list
// rather than interpreted by Go code — new step shapes don't need new
// columns.
type WorkflowStep struct {
	ID          uint `gorm:"primaryKey"`
	WorkflowID  uint `gorm:"index"`
	StepOrder   int
	StepType    string // trigger | hook | action | service
	Title       string
	Description string
	Icon        string
	ConfigJSON  string

	CreatedAt time.Time
	UpdatedAt time.Time
}
