// Package contracts defines the Flightctl API client interface contract.
//
// This interface abstracts the Flightctl REST API for testing and implementation.
// The actual implementation will use the Flightctl Go client library.
package contracts

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// FlightctlClient defines the interface for interacting with the Flightctl API.
// This abstraction enables:
// - Mocking for unit tests
// - Multiple implementations (REST, gRPC, mock)
// - API version compatibility layer
type FlightctlClient interface {
	// Device Management
	DeviceManager

	// Fleet Management
	FleetManager

	// Workload Management
	WorkloadManager

	// Health check
	Ping(ctx context.Context) error
}

// DeviceManager handles edge device operations.
type DeviceManager interface {
	// ListDevices retrieves devices, optionally filtered by fleet and/or labels.
	// Parameters:
	// - fleetID: If non-empty, filter to devices in this fleet
	// - labels: If non-empty, filter to devices matching all labels (AND logic)
	// Returns list of devices matching filters.
	ListDevices(ctx context.Context, fleetID string, labels map[string]string) ([]*Device, error)

	// GetDevice retrieves a specific device by ID.
	// Returns NotFound error if device doesn't exist.
	GetDevice(ctx context.Context, deviceID string) (*Device, error)

	// WatchDevices establishes a watch stream for device changes.
	// Optional: Implement if Flightctl supports watch API, otherwise poll with ListDevices.
	// Returns channel that emits DeviceEvent on device state changes.
	WatchDevices(ctx context.Context, fleetID string) (<-chan DeviceEvent, error)
}

// FleetManager handles fleet operations.
type FleetManager interface {
	// ListFleets retrieves all fleets.
	ListFleets(ctx context.Context) ([]*Fleet, error)

	// GetFleet retrieves a specific fleet by ID.
	// Returns NotFound error if fleet doesn't exist.
	GetFleet(ctx context.Context, fleetID string) (*Fleet, error)
}

// WorkloadManager handles workload deployment and lifecycle.
type WorkloadManager interface {
	// DeployWorkload deploys a new workload to a device.
	// Parameters:
	// - deviceID: Target device
	// - spec: Workload specification (containers, resources, etc.)
	// Returns workload ID assigned by Flightctl.
	DeployWorkload(ctx context.Context, deviceID string, spec *WorkloadSpec) (string, error)

	// UpdateWorkload updates an existing workload.
	// Implementation: Delete old workload, deploy new one (simple replace).
	// Parameters:
	// - deviceID: Device running the workload
	// - workloadID: Existing workload to replace
	// - spec: New workload specification
	UpdateWorkload(ctx context.Context, deviceID, workloadID string, spec *WorkloadSpec) error

	// DeleteWorkload removes a workload from a device.
	// Idempotent: No error if workload already deleted.
	DeleteWorkload(ctx context.Context, deviceID, workloadID string) error

	// GetWorkloadStatus retrieves current status of a workload.
	// Returns NotFound error if workload doesn't exist.
	GetWorkloadStatus(ctx context.Context, deviceID, workloadID string) (*WorkloadStatus, error)

	// ListWorkloads retrieves all workloads on a device.
	ListWorkloads(ctx context.Context, deviceID string) ([]*WorkloadStatus, error)
}

// Device represents an edge device in Flightctl.
type Device struct {
	ID          string            // Unique device identifier
	Name        string            // Human-readable name
	FleetID     string            // Fleet membership
	Labels      map[string]string // Device labels

	// Capacity
	Capacity    ResourceCapacity  // Total device resources
	Allocatable ResourceCapacity  // Available resources

	// Status
	Status      DeviceStatus      // Current state
	LastSeen    time.Time         // Last heartbeat from device
	Conditions  []DeviceCondition // Detailed status conditions

	// Metadata
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ResourceCapacity defines device resource capacity.
type ResourceCapacity struct {
	CPU    resource.Quantity // CPU cores
	Memory resource.Quantity // Memory bytes
}

// DeviceStatus represents device operational state.
type DeviceStatus struct {
	Phase   string // "Online", "Offline", "Unknown"
	Message string // Human-readable status
	Reason  string // Machine-readable reason code
}

// DeviceCondition represents a specific aspect of device status.
type DeviceCondition struct {
	Type               string    // e.g., "Ready", "DiskPressure", "NetworkReachable"
	Status             string    // "True", "False", "Unknown"
	LastTransitionTime time.Time
	Reason             string
	Message            string
}

// DeviceEvent represents a change to a device (for watch API).
type DeviceEvent struct {
	Type   string  // "ADDED", "MODIFIED", "DELETED"
	Device *Device
}

// Fleet represents a logical grouping of devices.
type Fleet struct {
	ID          string            // Unique fleet identifier
	Name        string            // Human-readable name
	Labels      map[string]string // Fleet labels
	DeviceCount int               // Number of devices in fleet

	// Metadata
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkloadSpec defines a workload to be deployed to a device.
// This maps from Kubernetes Pod spec to Flightctl workload format.
type WorkloadSpec struct {
	// Identity
	Name      string            // Workload name (from pod name)
	Namespace string            // Kubernetes namespace
	Labels    map[string]string // Workload labels

	// Containers
	Containers []ContainerSpec

	// Resources (aggregated from containers)
	Resources ResourceRequests

	// Metadata for tracking
	Annotations map[string]string // Store pod UID, etc.
}

// ContainerSpec defines a single container in a workload.
type ContainerSpec struct {
	Name       string
	Image      string
	Command    []string
	Args       []string
	Env        []EnvVar
	Resources  ResourceRequests
}

// EnvVar represents an environment variable.
type EnvVar struct {
	Name  string
	Value string
}

// ResourceRequests defines resource requests for a container or workload.
type ResourceRequests struct {
	CPU    resource.Quantity
	Memory resource.Quantity
}

// WorkloadStatus represents the current state of a deployed workload.
type WorkloadStatus struct {
	// Identity
	ID        string // Workload ID from Flightctl
	Name      string
	Namespace string
	DeviceID  string

	// Status
	Phase      string // "Pending", "Running", "Succeeded", "Failed", "Unknown"
	Message    string
	Reason     string
	Conditions []WorkloadCondition

	// Container status
	Containers []ContainerStatus

	// Timestamps
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// WorkloadCondition represents a specific aspect of workload status.
type WorkloadCondition struct {
	Type               string // e.g., "PodScheduled", "ContainersReady", "Ready"
	Status             string // "True", "False", "Unknown"
	LastTransitionTime time.Time
	Reason             string
	Message            string
}

// ContainerStatus represents the status of a single container.
type ContainerStatus struct {
	Name  string
	State ContainerState
	Ready bool

	// Restart tracking
	RestartCount int
	LastExitCode *int
}

// ContainerState represents the state of a container.
type ContainerState struct {
	Waiting    *ContainerStateWaiting
	Running    *ContainerStateRunning
	Terminated *ContainerStateTerminated
}

// ContainerStateWaiting indicates container is waiting to start.
type ContainerStateWaiting struct {
	Reason  string // e.g., "ImagePullBackOff", "PodInitializing"
	Message string
}

// ContainerStateRunning indicates container is running.
type ContainerStateRunning struct {
	StartedAt time.Time
}

// ContainerStateTerminated indicates container has terminated.
type ContainerStateTerminated struct {
	ExitCode   int
	Reason     string // e.g., "Completed", "Error", "OOMKilled"
	Message    string
	StartedAt  time.Time
	FinishedAt time.Time
}

// FlightctlClientConfig defines configuration for Flightctl API client.
type FlightctlClientConfig struct {
	// API endpoint
	APIURL string // e.g., "https://flightctl.example.com"

	// Authentication
	AuthToken   string // Bearer token
	InsecureTLS bool   // Skip TLS verification (testing only)

	// Client behavior
	Timeout time.Duration // HTTP request timeout
}

// Error types returned by Flightctl client
var (
	// ErrNotFound indicates the requested resource doesn't exist
	ErrNotFound = &FlightctlError{Code: "NotFound", Message: "Resource not found"}

	// ErrConflict indicates a conflict (e.g., duplicate resource)
	ErrConflict = &FlightctlError{Code: "Conflict", Message: "Resource conflict"}

	// ErrInsufficientResources indicates device lacks capacity
	ErrInsufficientResources = &FlightctlError{Code: "InsufficientResources", Message: "Device has insufficient resources"}

	// ErrDeviceOffline indicates device is not reachable
	ErrDeviceOffline = &FlightctlError{Code: "DeviceOffline", Message: "Device is offline"}

	// ErrUnauthorized indicates authentication failed
	ErrUnauthorized = &FlightctlError{Code: "Unauthorized", Message: "Authentication failed"}
)

// FlightctlError represents an error from the Flightctl API.
type FlightctlError struct {
	Code    string // Machine-readable error code
	Message string // Human-readable message
	Details string // Additional details
}

func (e *FlightctlError) Error() string {
	if e.Details != "" {
		return e.Code + ": " + e.Message + " (" + e.Details + ")"
	}
	return e.Code + ": " + e.Message
}

// NewFlightctlClient creates a new Flightctl API client.
// This is a factory function - actual implementation will be in pkg/flightctl/.
func NewFlightctlClient(config FlightctlClientConfig) (FlightctlClient, error) {
	panic("not implemented - contract definition only")
}
