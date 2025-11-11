# Research: Virtual Kubelet + Flightctl Integration

**Feature**: Non-Native Kubernetes Workload Deployment
**Date**: 2025-10-06
**Status**: Complete

## Overview
This document captures research findings and technical decisions for implementing a Virtual Kubelet provider that integrates with Flightctl to deploy Kubernetes workloads to edge devices.

---

## 1. Virtual Kubelet Provider Interface

### Decision
Implement the Virtual Kubelet `PodLifecycleHandler` and `NodeProvider` interfaces from the `virtual-kubelet/virtual-kubelet` Go module.

### Rationale
- Standard Virtual Kubelet provider pattern used by all major providers (Azure ACI, AWS Fargate, etc.)
- Well-documented interface with clear separation of concerns
- Enables the provider to masquerade as a kubelet node in the cluster
- Provides built-in support for pod status updates and node heartbeats

### Core Interfaces to Implement
**PodLifecycleHandler**:
- `CreatePod(ctx, pod)` - Deploy workload to target edge device via Flightctl
- `UpdatePod(ctx, pod)` - Update existing workload (simple replace strategy)
- `DeletePod(ctx, pod)` - Remove workload from edge device
- `GetPod(ctx, namespace, name)` - Retrieve pod status from edge device
- `GetPods(ctx)` - List all pods managed by this provider

**NodeProvider**:
- `Ping(ctx)` - Health check for the provider
- `NotifyNodeStatus(ctx, callback)` - Register callback for node status updates
- `GetNode(ctx)` - Return node object representing edge devices

### Alternatives Considered
- **Custom Kubernetes controller**: More flexibility but requires implementing full kubelet protocol, significantly more complex
- **CRD-based approach**: Would not appear as native Kubernetes nodes, breaking standard kubectl workflows

### Implementation Notes
- Each Virtual Kubelet provider instance can represent one or more edge devices
- Provider uses Kubernetes informers to watch for pod events
- Node status should aggregate capacity from Flightctl-managed devices
- Pod status updates should be polled from Flightctl API

---

## 2. Flightctl Integration Pattern

### Decision
Use Flightctl's REST API client library with declarative device configuration approach.

### Rationale
- Flightctl is designed for declarative fleet management, aligning with Kubernetes principles
- REST API provides standard HTTP-based integration
- Client library (if available) simplifies authentication and API versioning
- Declarative approach matches Kubernetes reconciliation patterns

### Integration Architecture
1. **Device Discovery**: Query Flightctl API for devices in managed fleets
2. **Label/Selector Mapping**: Map Kubernetes pod nodeSelectors to Flightctl device labels
3. **Workload Translation**: Convert Kubernetes Pod specs to Flightctl device configurations
4. **Status Synchronization**: Poll Flightctl for device/workload status, update pod status in Kubernetes

### API Client Design
```go
type FlightctlClient interface {
    // Device management
    ListDevices(ctx, fleetID, labels) ([]Device, error)
    GetDevice(ctx, deviceID) (Device, error)

    // Workload management
    DeployWorkload(ctx, deviceID, workloadSpec) error
    UpdateWorkload(ctx, deviceID, workloadSpec) error
    DeleteWorkload(ctx, deviceID, workloadID) error
    GetWorkloadStatus(ctx, deviceID, workloadID) (WorkloadStatus, error)

    // Fleet management
    ListFleets(ctx) ([]Fleet, error)
    GetFleet(ctx, fleetID) (Fleet, error)
}
```

### Alternatives Considered
- **gRPC API**: May be available in Flightctl but REST is more universally supported
- **Direct device SSH**: Would bypass Flightctl's declarative management, violating design principle
- **Message queue pattern**: Adds infrastructure complexity, not needed for initial scale (500 devices)

### Implementation Notes
- Implement retry logic with exponential backoff for Flightctl API calls
- Cache device metadata to reduce API calls
- Use Flightctl's watch/stream API if available for real-time status updates
- Authenticate using bearer tokens or mutual TLS (determine from Flightctl docs)

---

## 3. State Reconciliation Pattern

### Decision
Implement level-triggered reconciliation loop with periodic status polling.

### Rationale
- Matches Kubernetes controller pattern (observe, diff, act)
- Handles transient failures gracefully
- Works with intermittent connectivity to edge devices
- Aligns with Flightctl's declarative model

### Reconciliation Logic
1. **Observe**: Get desired state (Kubernetes pod spec) and actual state (Flightctl workload status)
2. **Diff**: Compare desired vs actual state
3. **Act**:
   - If pod exists in K8s but not in Flightctl → Deploy workload
   - If pod updated in K8s → Replace workload (per simple replace strategy)
   - If pod deleted in K8s → Remove workload from device
   - If device status changed → Update pod status in K8s

### Polling Strategy
- Poll Flightctl every 30 seconds for workload status (configurable)
- Use informer resync period (5 minutes) for full reconciliation
- Immediate reconciliation triggered by pod events from Kubernetes

### Alternatives Considered
- **Edge-triggered (event-only)**: Misses events during downtime, no self-healing
- **Watch-based**: Ideal but requires Flightctl watch API, fallback to polling if unavailable
- **Webhook-based**: Requires Flightctl to call back, adds network complexity

### Implementation Notes
- Use `workqueue` package for rate-limiting and retry
- Implement exponential backoff for failed reconciliations
- Add jitter to polling to avoid thundering herd with multiple providers

---

## 4. Resource Validation

### Decision
Validate CPU and memory requests against aggregated device capacity before pod admission.

### Rationale
- Prevents overcommitting edge devices with limited resources
- Matches clarified requirement (CPU/memory validation only)
- Fails fast with clear error messages
- Aligns with Kubernetes resource management model

### Validation Logic
```
When CreatePod called:
1. Extract resource requests from pod.spec.containers[*].resources.requests
2. Query Flightctl for target device (based on labels/selectors)
3. Get device capacity (CPU, memory) and current allocations
4. If requests > available: Reject with "Insufficient resources" error
5. If requests <= available: Proceed with deployment
```

### Capacity Tracking
- Maintain in-memory cache of device capacity (refreshed every 5 minutes)
- Track pod allocations per device
- Update allocations on pod create/delete events
- Handle races with optimistic locking (refresh and retry on conflict)

### Alternatives Considered
- **No validation**: Would cause device failures and poor user experience
- **Storage validation**: Deferred per clarifications (CPU/memory only for v1)
- **Runtime compatibility checks**: Deferred (assume all devices support containerization)

### Implementation Notes
- Use Kubernetes resource.Quantity for CPU/memory arithmetic
- Return clear error messages: "Device xyz has 2 CPU available, requested 4 CPU"
- Consider overcommit ratio (e.g., 1.5x) as future enhancement

---

## 5. Device Connectivity and Failure Handling

### Decision
Implement timeout-based reconnection with automatic workload rescheduling per clarified requirements.

### Rationale
- Matches clarification: "Wait for reconnection with timeout, then reschedule"
- Handles intermittent edge connectivity gracefully
- Balances availability (reschedule) with stability (don't reschedule too quickly)
- Configurable timeout allows tuning per deployment environment

### Timeout Strategy
```
When device becomes unavailable:
1. Mark pod status as "Unknown" with reason "DeviceDisconnected"
2. Start reconnection timeout timer (default: 5 minutes, configurable)
3. Poll device status every 30 seconds
4. If device reconnects before timeout:
   - Update pod status to actual state
   - Reconcile workload if needed
5. If timeout expires:
   - Delete pod (triggers Kubernetes to reschedule to another node/device)
   - Log reason: "Device exceeded reconnection timeout"
```

### Configuration
- Timeout configurable via provider environment variable: `DEVICE_RECONNECT_TIMEOUT`
- Default: 5 minutes
- Minimum: 1 minute (prevent flapping)
- Maximum: 30 minutes (balance availability vs. waiting time)

### Alternatives Considered
- **Immediate rescheduling**: Too aggressive, causes workload churn for transient network issues
- **No rescheduling**: Violates availability requirements
- **Exponential timeout**: Complexity not justified for v1

### Implementation Notes
- Use `time.AfterFunc` for timeout handling
- Cancel timeout when device reconnects
- Emit events to pod: "DeviceDisconnected", "DeviceReconnected", "DeviceTimeoutExceeded"
- Add Prometheus metrics: device_disconnection_total, workload_rescheduled_total

---

## 6. Pod to Device Targeting

### Decision
Implement label selector matching with fleet-level targeting support.

### Rationale
- Matches clarification: "Combination: labels, selectors, and fleet targeting"
- Aligns with Kubernetes scheduling semantics
- Provides flexibility (target specific devices or entire fleets)
- Enables refinement (fleet + labels for subset selection)

### Targeting Logic
```
When pod scheduled to Virtual Kubelet node:
1. Extract nodeSelector and/or node affinity from pod spec
2. If "flightctl.io/fleet" label present:
   - Query Flightctl for devices in that fleet
   - Filter by additional label selectors if present
3. Else if device labels present:
   - Query Flightctl for devices matching labels
4. Select device from candidate list:
   - Prefer device with most available resources
   - Tie-break by lowest workload count
5. Deploy workload to selected device
```

### Label Schema
- `flightctl.io/fleet: <fleet-name>` - Target specific fleet
- `flightctl.io/device-id: <device-id>` - Target specific device (for debugging/testing)
- Custom labels from Flightctl device metadata - Filter devices by capabilities

### Alternatives Considered
- **Fleet-only targeting**: Too coarse-grained, can't refine selection
- **Device ID only**: Too brittle, requires knowing specific device names
- **Kubernetes scheduler integration**: Complex, not needed for edge use case

### Implementation Notes
- Cache Flightctl device metadata to reduce API calls
- Refresh cache on device addition/removal events
- Handle case where no devices match selectors: Return error "No matching devices found"
- Consider device affinity (keep pods from same deployment on same device) as future enhancement

---

## 7. Authentication and Authorization

### Decision
Use Kubernetes RBAC for operator permissions, inherit Flightctl authentication from environment.

### Rationale
- Matches clarification: "Kubernetes RBAC only"
- Leverages existing cluster authentication/authorization
- Flightctl credentials managed via secrets, outside RBAC scope
- Separates concerns: K8s RBAC for who can deploy, Flightctl auth for provider access

### RBAC Design
Provider needs:
- Read pods (watch for scheduling to Virtual Kubelet nodes)
- Update pod status
- Read nodes (self-discovery)
- Update node status

Users need (standard Kubernetes RBAC):
- Create/update/delete pods (to deploy workloads)
- Read pod status (to view deployment status)

### Flightctl Authentication
- Provider authenticates to Flightctl using bearer token or client certificate
- Credentials stored in Kubernetes Secret, mounted to provider pod
- Token refresh handled automatically (if JWT-based)

### Alternatives Considered
- **Separate RBAC for Flightctl**: Adds complexity, not required by spec
- **Device-level auth**: Flightctl manages this, outside provider scope
- **No authentication**: Insecure, not acceptable

### Implementation Notes
- Create ServiceAccount for provider deployment
- Bind ClusterRole with required permissions
- Mount Flightctl credentials via Secret volume
- Validate credentials on provider startup, fail fast if invalid

---

## 8. Testing Strategy

### Decision
Three-tier testing: unit tests (reconciler logic), integration tests (kind + mock Flightctl), contract tests (API schemas).

### Rationale
- Unit tests ensure reconciliation logic correctness
- Integration tests validate end-to-end workflows in real Kubernetes
- Contract tests prevent API breaking changes
- Aligns with TDD constitutional principle

### Test Levels

**Unit Tests** (tests/unit/):
- Provider interface methods (CreatePod, UpdatePod, DeletePod, etc.)
- Reconciler logic (state diffing, timeout handling)
- Resource validation (capacity checks)
- Label matching and device selection

**Integration Tests** (tests/integration/):
- Deploy kind cluster
- Deploy mock Flightctl server (or use Flightctl test instance)
- Deploy Virtual Kubelet provider
- Test scenarios:
  - Pod deployment to edge device
  - Pod update (simple replace)
  - Device disconnection and reconnection
  - Workload rescheduling after timeout
  - Fleet-level targeting

**Contract Tests** (tests/contract/):
- Flightctl API client request/response schemas
- Virtual Kubelet provider interface compliance

### Test Data
- Use testdata/ directory for pod specs, device configs
- Mock Flightctl server returns predefined device lists and status
- Simulate network failures with context cancellation

### Alternatives Considered
- **Manual testing only**: Not sustainable, violates TDD principle
- **End-to-end tests only**: Slow, hard to isolate failures
- **No contract tests**: Risk of integration breakage

### Implementation Notes
- Use `kind` (Kubernetes in Docker) for integration tests
- Use `httptest` for mock Flightctl server
- Use table-driven tests for reconciler logic
- Add test coverage requirements (>80% for unit tests)

---

## 9. Observability

### Decision
Implement structured logging (logr) and Prometheus metrics for device and workload state.

### Rationale
- Structured logging enables debugging in production
- Prometheus metrics enable monitoring and alerting
- Aligns with observability constitutional principle
- Standard in Kubernetes ecosystem

### Logging
Use `logr` interface with structured fields:
- `pod`: namespace/name
- `device`: device ID
- `fleet`: fleet name
- `operation`: create/update/delete/reconcile
- `error`: error message if operation failed

Log levels:
- Info: Normal operations (pod created, device connected)
- Error: Failures (API errors, validation failures)
- Debug: Detailed reconciliation steps (disabled in production)

### Metrics
Prometheus metrics to track:
```
# Workload metrics
vk_flightctl_pods_total{status="running|pending|failed"}
vk_flightctl_pod_deployment_duration_seconds{device}
vk_flightctl_pod_operations_total{operation="create|update|delete", status="success|failure"}

# Device metrics
vk_flightctl_devices_total{fleet, status="connected|disconnected"}
vk_flightctl_device_capacity{fleet, device, resource="cpu|memory"}
vk_flightctl_device_allocatable{fleet, device, resource="cpu|memory"}

# Reconciliation metrics
vk_flightctl_reconcile_duration_seconds{operation}
vk_flightctl_reconcile_errors_total{operation, error_type}

# Timeout metrics
vk_flightctl_device_disconnections_total{fleet, device}
vk_flightctl_workload_rescheduled_total{fleet, reason}
```

### Alternatives Considered
- **No observability**: Impossible to debug production issues
- **Plain text logs**: Hard to parse and analyze
- **Custom metrics system**: Reinventing the wheel, Prometheus is standard

### Implementation Notes
- Export metrics on /metrics endpoint (standard Prometheus scrape target)
- Use controller-runtime's logging framework (logr)
- Add tracing (OpenTelemetry) as future enhancement for distributed debugging

---

## 10. Deployment Model

### Decision
Deploy Virtual Kubelet provider as a Kubernetes Deployment with replica count = 1 per fleet or device group.

### Rationale
- Kubernetes-native deployment
- Managed lifecycle (restarts, updates)
- Single replica prevents split-brain scenarios
- Can scale horizontally by running multiple providers (each managing different fleets)

### Deployment Configuration
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vk-flightctl-provider
spec:
  replicas: 1
  template:
    spec:
      serviceAccountName: vk-flightctl-provider
      containers:
      - name: provider
        image: vk-flightctl-provider:latest
        env:
        - name: FLIGHTCTL_API_URL
          value: "https://flightctl.example.com"
        - name: FLIGHTCTL_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: flightctl-credentials
              key: token
        - name: DEVICE_RECONNECT_TIMEOUT
          value: "5m"
        - name: FLEET_ID
          value: "edge-fleet-1"  # Optional: limit to specific fleet
```

### High Availability
- v1: Single replica per fleet (acceptable for 500 device scale)
- Future: Leader election for active/standby HA deployment

### Alternatives Considered
- **DaemonSet**: No benefit since provider doesn't run on every node
- **StatefulSet**: Not needed, provider is stateless (state in Kubernetes and Flightctl)
- **Multiple replicas**: Requires leader election, adds complexity for v1

### Implementation Notes
- Use liveness probe: `/healthz` endpoint
- Use readiness probe: Check Flightctl API connectivity
- Set resource requests/limits (2 CPU, 4Gi memory for 500 devices)
- Configure PodDisruptionBudget for safe updates

---

## Unresolved Items (For Future Phases)

The following items from the spec remain as "NEEDS CLARIFICATION" and are deferred:
1. **Offline operation requirements**: How long can devices operate without control plane? Local state persistence?
2. **Compatibility checking**: Beyond CPU/memory, validate container runtime, kernel versions?
3. **Device lifecycle policies**: Auto-removal of decommissioned devices? Graceful drain?
4. **Device metadata exposure** (FR-015): Which fields should be exposed to operators?

**Recommendation**: Address during implementation if needed, or defer to v2 based on user feedback.

---

## Summary

All critical technical decisions have been researched and documented. The approach follows standard Kubernetes operator and Virtual Kubelet patterns:

- **Provider Interface**: Virtual Kubelet PodLifecycleHandler + NodeProvider
- **Integration**: Flightctl REST API with declarative workload management
- **Reconciliation**: Level-triggered control loop with periodic polling
- **Validation**: CPU/memory capacity checks
- **Failure Handling**: Timeout-based reconnection with automatic rescheduling
- **Targeting**: Label selectors + fleet identifiers
- **Auth**: Kubernetes RBAC for operator, Flightctl credentials via secrets
- **Testing**: Unit + integration + contract tests
- **Observability**: Structured logging + Prometheus metrics
- **Deployment**: Single-replica Kubernetes Deployment

**Next Phase**: Design data models, API contracts, and quickstart validation scenarios.
