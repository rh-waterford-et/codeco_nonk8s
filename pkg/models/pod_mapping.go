package models

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// PodDeviceMapping tracks which pod is running on which device.
// This is a simple index structure that replaces the complex Workload abstraction.
// Per Constitution Principle VII (Simplicity & Minimalism).
type PodDeviceMapping struct {
	PodKey     string    // namespace/name
	PodUID     types.UID // Kubernetes pod UID for uniqueness
	DeviceID   string    // Target device ID
	DeployedAt time.Time // When the pod was deployed
}

// NewPodDeviceMapping creates a new mapping.
func NewPodDeviceMapping(namespace, name string, uid types.UID, deviceID string) *PodDeviceMapping {
	return &PodDeviceMapping{
		PodKey:     namespace + "/" + name,
		PodUID:     uid,
		DeviceID:   deviceID,
		DeployedAt: time.Now(),
	}
}
