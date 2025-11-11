package models

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// ReconciliationRecord tracks reconciliation attempts for debugging.
type ReconciliationRecord struct {
	Timestamp       time.Time
	PodKey          string // namespace/name
	Operation       ReconcileOperation
	DesiredState    corev1.PodPhase
	ActualState     corev1.PodPhase
	Action          ReconcileAction
	Result          ReconcileResult
	ErrorMessage    string
	DurationSeconds float64
}

// ReconcileOperation represents the type of reconciliation operation.
type ReconcileOperation string

const (
	ReconcileCreate ReconcileOperation = "Create"
	ReconcileUpdate ReconcileOperation = "Update"
	ReconcileDelete ReconcileOperation = "Delete"
	ReconcileStatus ReconcileOperation = "Status"
)

// ReconcileAction represents the action taken during reconciliation.
type ReconcileAction string

const (
	ActionNone    ReconcileAction = "None"
	ActionDeploy  ReconcileAction = "Deploy"
	ActionReplace ReconcileAction = "Replace"
	ActionRemove  ReconcileAction = "Remove"
	ActionUpdate  ReconcileAction = "Update"
)

// ReconcileResult represents the result of a reconciliation.
type ReconcileResult string

const (
	ResultSuccess ReconcileResult = "Success"
	ResultFailed  ReconcileResult = "Failed"
	ResultRetry   ReconcileResult = "Retry"
)
