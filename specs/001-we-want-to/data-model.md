# Data Model: Virtual Kubelet Flightctl Provider

**Feature**: Non-Native Kubernetes Workload Deployment
**Date**: 2025-10-06

## Overview
This document defines the data models used by the Virtual Kubelet Flightctl provider to represent edge devices, fleets, workloads, and their status.

---

## Core Entities

### 1. Edge Device

Represents a physical or virtual edge device managed by Flightctl that can run containerized workloads.

**Fields**:
```go
type Device struct {
    // Identity
    ID          string            // Unique device identifier from Flightctl
    Name        string            // Human-readable device name
    FleetID     string            // Fleet this device belongs to
    Labels      map[string]string // Device labels for targeting/filtering

    // Capacity
    Capacity    ResourceList      // Total device resources
    Allocatable ResourceList      // Available resources after system overhead

    // Status
    Status         DeviceStatus      // Current device state
    LastHeartbeat  time.Time         // Last successful communication
    ConnectionState ConnectionState  // Connected, Disconnected, Unknown

    // Metadata
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type ResourceList struct {
    CPU    resource.Quantity  // CPU cores (e.g., "4" = 4 cores)
    Memory resource.Quantity  // Memory bytes (e.g., "8Gi")
}

type DeviceStatus struct {
    Phase   DevicePhase  // Ready, NotReady, Unknown
    Message string       // Human-readable status message
    Reason  string       // Machine-readable reason code
}

type DevicePhase string
const (
    DeviceReady    DevicePhase = "Ready"
    DeviceNotReady DevicePhase = "NotReady"
    DeviceUnknown  DevicePhase = "Unknown"
)

type ConnectionState string
const (
    Connected    ConnectionState = "Connected"
    Disconnected ConnectionState = "Disconnected"
    Unknown      ConnectionState = "Unknown"
)
```

**Relationships**:
- Belongs to one Fleet (many-to-one)
- Runs zero or more Pods (tracked via PodDeviceMapping)

**Validation Rules**:
- ID must be unique across all devices
- FleetID must reference an existing fleet
- Capacity.CPU and Capacity.Memory must be > 0
- Allocatable <= Capacity
- LastHeartbeat updated on every status check

**State Transitions**:
```
Ready → NotReady: When device becomes unreachable
NotReady → Ready: When device reconnects and passes health check
Ready/NotReady → Unknown: When connection state is unclear
Unknown → Ready: When device reconnects and is healthy
Unknown → NotReady: When device confirmed down
```

---

### 2. Fleet

Represents a logical grouping of edge devices managed collectively.

**Fields**:
```go
type Fleet struct {
    // Identity
    ID     string            // Unique fleet identifier from Flightctl
    Name   string            // Human-readable fleet name
    Labels map[string]string // Fleet labels for targeting

    // Metadata
    DeviceCount int       // Number of devices in fleet
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Relationships**:
- Contains zero or more Devices (one-to-many)

**Validation Rules**:
- ID must be unique across all fleets
- Name must be non-empty
- DeviceCount >= 0

---

### 3. Pod Tracking

**REMOVED**: The Workload abstraction has been eliminated per architectural simplification (Constitution Principle VII: Simplicity & Minimalism).

**New Approach**: The provider works directly with Kubernetes Pod objects:
- **Input**: Kubernetes Pod spec (v1.Pod) received via Virtual Kubelet PodLifecycleHandler
- **Output**: Pod status updates sent back to Kubernetes API
- **No intermediate data structure**: Pod spec is passed directly to Flightctl HTTP client
- **Status sync**: Flightctl workload status is mapped directly to Kubernetes Pod status

**Pod-to-Device Mapping**:
The provider maintains an in-memory index to track which pod is running on which device:

```go
// Simple tracking map (no separate Workload entity)
type PodDeviceMapping struct {
    PodKey    string // namespace/name
    PodUID    types.UID
    DeviceID  string
    DeployedAt time.Time
}
```

**Rationale**:
- Eliminates duplicate state management between Kubernetes Pod and internal Workload models
- Reduces code complexity and maintenance burden
- Pod spec already contains all necessary deployment information (containers, resources, labels)
- Status is canonical in Kubernetes; no need for intermediate representation

---

### 4. Deployment Target

Represents the specification for targeting devices when deploying workloads.

**Fields**:
```go
type DeploymentTarget struct {
    // Fleet targeting
    FleetID    *string // If set, target devices in this fleet

    // Label targeting
    Selectors  map[string]string // Label selectors (AND logic)

    // Device targeting (for debugging/testing)
    DeviceID   *string // If set, target specific device (overrides other fields)
}
```

**Usage**:
- Extracted from Kubernetes pod nodeSelector and node affinity
- Evaluated to produce a candidate device list
- Device selected from candidates based on resource availability

**Validation Rules**:
- At least one targeting field must be set (FleetID, Selectors, or DeviceID)
- If DeviceID is set, it must reference an existing device
- If FleetID is set, it must reference an existing fleet
- Selectors keys must follow Kubernetes label key format

**Selection Algorithm**:
```
1. Build candidate device list:
   - If DeviceID set: [single device]
   - If FleetID set: devices in fleet
   - If Selectors set: devices matching all selectors
   - If FleetID + Selectors: intersection of both

2. Filter candidates:
   - Remove devices with ConnectionState != Connected
   - Remove devices with insufficient Allocatable resources

3. Select device:
   - Sort by available resources (descending)
   - Tie-break by fewest existing workloads
   - Return first device

4. If no candidates: Return error "No suitable device found"
```

---

### 5. Device Status Snapshot

Represents a point-in-time snapshot of device status for reconciliation.

**Fields**:
```go
type DeviceStatusSnapshot struct {
    DeviceID        string
    Timestamp       time.Time
    Status          DeviceStatus
    ConnectionState ConnectionState
    Allocatable     ResourceList
    RunningPods     []PodSummary
}

type PodSummary struct {
    Namespace string
    Name      string
    UID       types.UID
    Phase     v1.PodPhase // Kubernetes pod phase
    Resources v1.ResourceRequirements
}
```

**Usage**:
- Cached for 30 seconds to reduce Flightctl API calls
- Refreshed on demand when pod events occur
- Used by reconciler to diff desired vs actual state

---

### 6. Reconciliation Record

Tracks reconciliation attempts for debugging and observability.

**Fields**:
```go
type ReconciliationRecord struct {
    Timestamp       time.Time
    PodKey          string // namespace/name
    Operation       ReconcileOperation
    DesiredState    v1.PodPhase
    ActualState     v1.PodPhase
    Action          ReconcileAction
    Result          ReconcileResult
    ErrorMessage    string
    DurationSeconds float64
}

type ReconcileOperation string
const (
    ReconcileCreate ReconcileOperation = "Create"
    ReconcileUpdate ReconcileOperation = "Update"
    ReconcileDelete ReconcileOperation = "Delete"
    ReconcileStatus ReconcileOperation = "Status"
)

type ReconcileAction string
const (
    ActionNone     ReconcileAction = "None"     // No action needed
    ActionDeploy   ReconcileAction = "Deploy"   // Deploy workload to device
    ActionReplace  ReconcileAction = "Replace"  // Replace existing workload
    ActionRemove   ReconcileAction = "Remove"   // Remove workload from device
    ActionUpdate   ReconcileAction = "Update"   // Update pod status
)

type ReconcileResult string
const (
    ResultSuccess ReconcileResult = "Success"
    ResultFailed  ReconcileResult = "Failed"
    ResultRetry   ReconcileResult = "Retry"
)
```

**Usage**:
- Logged for every reconciliation loop iteration
- Used to generate Prometheus metrics
- Retained in memory for last 100 records (circular buffer)
- Emitted via structured logging

---

### 7. Timeout Tracker

Tracks device disconnection timeouts for workload rescheduling.

**Fields**:
```go
type TimeoutTracker struct {
    DeviceID          string
    DisconnectedAt    time.Time
    TimeoutDuration   time.Duration
    TimeoutAt         time.Time
    AffectedPods      []string // Pod keys (namespace/name)
    TimerCancelFunc   context.CancelFunc
}
```

**Usage**:
- Created when device transitions to Disconnected state
- Timer fires after TimeoutDuration (default 5 minutes, configurable)
- On timeout: Delete affected pods (triggers Kubernetes rescheduling)
- Cancelled if device reconnects before timeout

**Validation Rules**:
- TimeoutDuration must be >= 1 minute
- TimeoutDuration must be <= 30 minutes
- DeviceID must reference an existing device

---

## Data Flows

### Pod Deployment Flow
```
1. Kubernetes schedules pod to Virtual Kubelet node
2. Provider receives CreatePod event with v1.Pod object
3. Extract DeploymentTarget from pod spec (nodeSelector, affinity)
4. Select device from candidate list
5. Validate device has sufficient resources
6. Pass Pod spec directly to Flightctl HTTP client
7. Flightctl client converts v1.Pod to Flightctl workload format and deploys
8. Flightctl client returns workload status
9. Map Flightctl status to v1.PodStatus and update in Kubernetes
10. Store PodDeviceMapping in memory for tracking
```

### Status Reconciliation Flow
```
1. Timer fires every 30 seconds
2. For each tracked Pod (from PodDeviceMapping index):
   a. Fetch DeviceStatusSnapshot from cache or Flightctl API
   b. Query Flightctl for pod's current status on device
   c. Map Flightctl status to v1.PodStatus
   d. If mismatch with Kubernetes pod status: Update pod in Kubernetes
   e. Create ReconciliationRecord for observability
```

### Device Disconnection Flow
```
1. Status poll detects device unreachable
2. Update Device.ConnectionState = Disconnected
3. Create TimeoutTracker with configured duration
4. Lookup affected pods via PodDeviceMapping
5. Update affected pod statuses to "Unknown" in Kubernetes
6. If device reconnects before timeout:
   a. Cancel TimeoutTracker
   b. Query Flightctl for current pod statuses
   c. Update pod statuses in Kubernetes accordingly
7. If timeout expires:
   a. Delete affected pods from Kubernetes (triggers rescheduling)
   b. Remove PodDeviceMapping entries
   c. Log rescheduling event
```

---

## Persistence

### Kubernetes State
- **Pods**: Stored in Kubernetes etcd by Kubernetes itself
- **Node**: Virtual node object stored in Kubernetes etcd
- Provider reads/writes via Kubernetes API

### Flightctl State
- **Devices**: Stored in Flightctl backend
- **Fleets**: Stored in Flightctl backend
- **Pod configurations**: Stored in Flightctl backend (converted from Kubernetes pod specs)
- Provider reads/writes via Flightctl API

### Provider State
- **Device metadata cache**: In-memory, refreshed every 5 minutes
- **DeviceStatusSnapshot cache**: In-memory, TTL 30 seconds
- **TimeoutTrackers**: In-memory, created/destroyed dynamically
- **ReconciliationRecords**: In-memory circular buffer (last 100)

**Note**: Provider is stateless - all persistent state lives in Kubernetes or Flightctl. Provider can be restarted without data loss.

---

## Indexes and Caching

For performance, the provider maintains in-memory indexes:

```go
type ProviderCache struct {
    // Device lookups
    DevicesByID    map[string]*Device
    DevicesByFleet map[string][]*Device

    // Pod-to-Device tracking (replaces Workload abstraction)
    PodToDevice    map[string]*PodDeviceMapping // key: namespace/name
    DeviceToPods   map[string][]string          // key: deviceID, value: pod keys

    // Status snapshots
    StatusSnapshots   map[string]*DeviceStatusSnapshot // TTL 30s

    // Timeout tracking
    ActiveTimeouts    map[string]*TimeoutTracker

    // Refresh timestamps
    LastDeviceRefresh time.Time
    LastFleetRefresh  time.Time
}
```

**Refresh Policy**:
- Devices: Every 5 minutes or on demand
- Fleets: Every 10 minutes or on demand
- Status: Every 30 seconds per device with active pods
- Timeouts: Event-driven (created/cancelled as needed)

---

## Summary

The data model centers on two main entities:
1. **Device** - Edge devices with capacity and connectivity state
2. **Fleet** - Logical groupings of devices

**Simplified Architecture** (Constitution Principle VII: Simplicity):
- **No Workload abstraction**: Provider works directly with Kubernetes v1.Pod objects
- **Direct pass-through**: Pod specs sent to Flightctl HTTP client, status mapped back to Pod
- **Minimal tracking**: Simple PodDeviceMapping index for device lookup

Supporting entities enable:
- **DeploymentTarget** - Device selection logic
- **DeviceStatusSnapshot** - Efficient status caching
- **ReconciliationRecord** - Observability and debugging
- **TimeoutTracker** - Failure handling and rescheduling

All persistent state lives in Kubernetes (pods, node) or Flightctl (devices, fleets, deployed pods). The provider maintains only transient in-memory caches and indexes for performance.
