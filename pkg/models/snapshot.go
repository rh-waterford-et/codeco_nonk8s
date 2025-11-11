package models

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DeviceStatusSnapshot represents a point-in-time snapshot of device status.
type DeviceStatusSnapshot struct {
	DeviceID        string
	Timestamp       time.Time
	Status          DeviceStatus
	ConnectionState ConnectionState
	Allocatable     ResourceList
	RunningPods     []PodSummary
}

// PodSummary contains essential pod information from device.
type PodSummary struct {
	Namespace string
	Name      string
	UID       types.UID
	Phase     corev1.PodPhase
	Resources corev1.ResourceRequirements
}

// IsExpired checks if the snapshot is older than the given TTL.
func (s *DeviceStatusSnapshot) IsExpired(ttl time.Duration) bool {
	return time.Since(s.Timestamp) > ttl
}
