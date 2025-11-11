// Package contracts defines the Virtual Kubelet provider interface contract
// that must be implemented by the Flightctl provider.
//
// This file serves as a contract test specification - the actual implementation
// will be in pkg/provider/.
package contracts

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodLifecycleHandler defines the interface for managing pod lifecycle on edge devices.
// This interface is defined by the Virtual Kubelet project and must be implemented
// by all providers.
type PodLifecycleHandler interface {
	// CreatePod deploys a new workload to an edge device based on the pod specification.
	// Returns error if:
	// - No suitable device found matching selectors
	// - Device lacks sufficient resources
	// - Flightctl API call fails
	CreatePod(ctx context.Context, pod *corev1.Pod) error

	// UpdatePod updates an existing workload on an edge device.
	// Implementation uses simple replace strategy: stop old, start new.
	// Returns error if:
	// - Pod not found on device
	// - Flightctl API call fails
	UpdatePod(ctx context.Context, pod *corev1.Pod) error

	// DeletePod removes a workload from an edge device.
	// Returns error if:
	// - Flightctl API call fails
	// Does NOT return error if pod already deleted (idempotent).
	DeletePod(ctx context.Context, pod *corev1.Pod) error

	// GetPod retrieves the current status of a pod running on an edge device.
	// Returns:
	// - Pod with updated status if found
	// - Nil, NotFound error if pod doesn't exist
	// - Nil, error if Flightctl API call fails
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

	// GetPods retrieves the status of all pods managed by this provider.
	// Returns list of pods with current status from edge devices.
	GetPods(ctx context.Context) ([]*corev1.Pod, error)

	// GetPodStatus retrieves just the status of a pod without full pod spec.
	// Used for efficient status polling.
	GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)
}

// NodeProvider defines the interface for managing the virtual node representation.
// The virtual node represents the aggregated capacity of edge devices managed
// by this provider instance.
type NodeProvider interface {
	// Ping checks if the provider is healthy and can communicate with Flightctl.
	// Returns error if Flightctl API is unreachable or provider is unhealthy.
	Ping(ctx context.Context) error

	// NotifyNodeStatus registers a callback to be invoked when node status changes.
	// The callback should be called whenever:
	// - Device capacity changes (devices added/removed from fleet)
	// - Device connectivity state changes
	// - Allocatable resources change significantly
	NotifyNodeStatus(ctx context.Context, callback func(*corev1.Node)) error

	// GetNode returns the current node object representing edge devices.
	// Node capacity represents aggregated capacity of all connected devices.
	// Node allocatable represents currently available resources.
	GetNode(ctx context.Context) (*corev1.Node, error)
}

// LogsProvider defines the interface for retrieving pod logs from edge devices.
// This is an optional interface - implementing it enables kubectl logs support.
type LogsProvider interface {
	// GetContainerLogs retrieves logs from a container running on an edge device.
	// Returns io.ReadCloser that streams logs from the device.
	// Options:
	// - tail: Number of lines from end of logs
	// - follow: Stream logs continuously
	// - timestamps: Include timestamps in log lines
	GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts ContainerLogsOpts) (io.ReadCloser, error)
}

// ContainerLogsOpts specifies options for retrieving container logs.
type ContainerLogsOpts struct {
	Tail       int
	Follow     bool
	Previous   bool
	Timestamps bool
}

// MetricsProvider defines the interface for retrieving pod metrics from edge devices.
// This is an optional interface - implementing it enables kubectl top pod support.
type MetricsProvider interface {
	// GetPodMetrics retrieves current resource usage for a pod.
	// Returns current CPU and memory usage from edge device.
	GetPodMetrics(ctx context.Context, namespace, podName string) (*PodMetrics, error)
}

// PodMetrics represents resource usage for a pod.
type PodMetrics struct {
	Namespace  string
	PodName    string
	Containers []ContainerMetrics
	Timestamp  int64 // Unix timestamp
}

// ContainerMetrics represents resource usage for a container.
type ContainerMetrics struct {
	Name   string
	CPU    resource.Quantity // CPU usage in cores
	Memory resource.Quantity // Memory usage in bytes
}

// ProviderConfig defines configuration for the Flightctl provider.
// This configuration is loaded from environment variables and secrets.
type ProviderConfig struct {
	// Flightctl API configuration
	FlightctlAPIURL      string // Flightctl API endpoint URL
	FlightctlAuthToken   string // Bearer token for authentication
	FlightctlInsecureTLS bool   // Skip TLS verification (for testing only)

	// Provider behavior configuration
	DeviceReconnectTimeout  string // Duration string (e.g., "5m")
	StatusPollInterval      string // Duration string (e.g., "30s")
	DeviceRefreshInterval   string // Duration string (e.g., "5m")
	StatusCacheTTL          string // Duration string (e.g., "30s")

	// Fleet targeting (optional)
	FleetID string // If set, only manage devices in this fleet

	// Virtual node configuration
	NodeName            string // Name of the virtual node
	OperatingSystem     string // OS reported by node (default: "Linux")
	NodeLabels          map[string]string // Additional labels for the node
	NodeAnnotations     map[string]string // Additional annotations
}

// ProviderMetrics defines Prometheus metrics exposed by the provider.
// These metrics are scraped by Prometheus for monitoring and alerting.
type ProviderMetrics interface {
	// Pod operation metrics
	RecordPodOperation(operation, status string, duration float64)
	RecordPodCount(status string, count int)

	// Device metrics
	RecordDeviceCount(fleet, status string, count int)
	RecordDeviceCapacity(fleet, device, resource string, quantity float64)
	RecordDeviceAllocatable(fleet, device, resource string, quantity float64)

	// Reconciliation metrics
	RecordReconcileDuration(operation string, duration float64)
	RecordReconcileError(operation, errorType string)

	// Timeout metrics
	RecordDeviceDisconnection(fleet, device string)
	RecordWorkloadRescheduled(fleet, reason string)
}
