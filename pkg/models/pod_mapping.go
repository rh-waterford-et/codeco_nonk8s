package models

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PodDeviceMapping tracks which pod is running on which device.
// This is a simple index structure that replaces the complex Workload abstraction.
// Per Constitution Principle VII (Simplicity & Minimalism).
type PodDeviceMapping struct {
	PodKey     string            // namespace/name
	Namespace  string            // Pod namespace (extracted for convenience)
	Name       string            // Pod name (extracted for convenience)
	PodUID     types.UID         // Kubernetes pod UID for uniqueness
	DeviceID   string            // Target device ID
	DeployedAt time.Time         // When the pod was deployed
	Status     *corev1.PodStatus // Cached pod status (nil if not yet fetched)
}

// NewPodDeviceMapping creates a new mapping.
func NewPodDeviceMapping(namespace, name string, uid types.UID, deviceID string) *PodDeviceMapping {
	return &PodDeviceMapping{
		PodKey:     namespace + "/" + name,
		Namespace:  namespace,
		Name:       name,
		PodUID:     uid,
		DeviceID:   deviceID,
		DeployedAt: time.Now(),
		Status:     nil, // Status will be set after deployment
	}
}
