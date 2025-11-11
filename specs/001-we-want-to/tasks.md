# Tasks: Non-Native Kubernetes Workload Deployment

**Input**: Design documents from `/home/lgriffin/Documents/claude_testing/eu-example/specs/001-we-want-to/`
**Prerequisites**: plan.md, research.md, data-model.md, contracts/, quickstart.md
**Tech Stack**: Go 1.21+, Virtual Kubelet SDK, Flightctl client, Kubernetes client-go, controller-runtime
**Project Type**: Single Kubernetes operator (cmd/, pkg/, tests/, config/)

## Execution Flow (main)
```
1. Load plan.md from feature directory ✓
   → Tech stack: Go 1.21+, Virtual Kubelet, Flightctl
   → Structure: cmd/, pkg/, tests/, config/
2. Load optional design documents: ✓
   → data-model.md: 7 entities extracted
   → contracts/: 2 contract files found
   → research.md: 10 technical decisions extracted
   → quickstart.md: 6 test scenarios identified
3. Generate tasks by category:
   → Setup: Project init, dependencies, Go modules
   → Tests: Contract tests (2), integration tests (6)
   → Core: Models (7), Flightctl client (4), Provider (5), Reconciler (3)
   → Integration: RBAC, deployment manifests, observability
   → Polish: Unit tests, performance validation, docs
4. Apply task rules:
   → Different files = mark [P] for parallel (27 tasks)
   → Same file = sequential (13 tasks)
   → Tests before implementation (TDD enforced)
5. Number tasks sequentially (T001-T040)
6. Generate dependency graph (see Dependencies section)
7. Create parallel execution examples (see Parallel Example)
8. Validate task completeness:
   ✓ All contracts have tests
   ✓ All entities have models
   ✓ All quickstart scenarios covered
9. Return: SUCCESS (40 tasks ready for execution)
```

---

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- All file paths are absolute from repository root
- Dependencies noted in each task

---

## Phase 3.1: Setup

- [x] **T001** Create Go project structure with directories: `cmd/vk-flightctl-provider/`, `pkg/{provider,flightctl,reconciler,models}/`, `tests/{unit,integration}/`, `config/`

- [x] **T002** Initialize Go module and install dependencies:
  ```bash
  go mod init github.com/yourorg/vk-flightctl-provider
  go get github.com/virtual-kubelet/virtual-kubelet@latest
  go get k8s.io/client-go@v0.28.0
  go get sigs.k8s.io/controller-runtime@v0.16.0
  go get k8s.io/api@v0.28.0
  go get k8s.io/apimachinery@v0.28.0
  ```
  Reference: plan.md Technical Context, research.md Section 1

- [x] **T003** [P] Configure linting and formatting:
  - Create `.golangci.yml` with standard linters (govet, staticcheck, gofmt)
  - Add `Makefile` with targets: `lint`, `fmt`, `test`, `build`
  - Add GitHub Actions workflow `.github/workflows/ci.yml` (or equivalent CI)

---

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3

**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

### Contract Tests

- [ ] **T004** [P] Contract test for Virtual Kubelet PodLifecycleHandler interface in `tests/unit/provider_contract_test.go`:
  - Test CreatePod method signature and error cases
  - Test UpdatePod method signature and error cases
  - Test DeletePod method signature and error cases
  - Test GetPod, GetPods method signatures
  - Use mock implementations (tests should fail - no real implementation yet)
  - Reference: contracts/virtual-kubelet-provider.go

- [ ] **T005** [P] Contract test for Virtual Kubelet NodeProvider interface in `tests/unit/node_contract_test.go`:
  - Test Ping method
  - Test NotifyNodeStatus callback registration
  - Test GetNode method and node object structure
  - Use mock implementations (tests should fail)
  - Reference: contracts/virtual-kubelet-provider.go

- [ ] **T006** [P] Contract test for Flightctl DeviceManager interface in `tests/unit/flightctl_device_contract_test.go`:
  - Test ListDevices with fleet and label filters
  - Test GetDevice by ID
  - Test error handling (NotFound, Unauthorized, DeviceOffline)
  - Use mock HTTP server (tests should fail)
  - Reference: contracts/flightctl-client.go

- [ ] **T007** [P] Contract test for Flightctl PodManager interface in `tests/unit/flightctl_pod_contract_test.go`:
  - Test DeployPod method (accepts v1.Pod, returns status)
  - Test UpdatePod (simple replace)
  - Test DeletePod (idempotent)
  - Test GetPodStatus and ListPods
  - Use mock HTTP server (tests should fail)
  - Reference: contracts/flightctl-client.go

### Integration Tests (from quickstart.md scenarios)

- [ ] **T008** [P] Integration test: Deploy workload to edge device in `tests/integration/deployment_test.go`:
  - Setup: kind cluster + mock Flightctl server
  - Test: Create pod targeting virtual node
  - Assert: Pod reaches Running state within 10s
  - Assert: Workload appears in mock Flightctl device
  - Reference: quickstart.md Scenario 1
  - Dependencies: T004-T007 must fail first (TDD)

- [ ] **T009** [P] Integration test: Fleet-level targeting in `tests/integration/fleet_test.go`:
  - Setup: Virtual node with fleet label
  - Test: Create deployment with 3 replicas, nodeSelector fleet=edge-fleet-1
  - Assert: Pods distributed across devices in fleet
  - Assert: Node allocatable resources updated
  - Reference: quickstart.md Scenario 2
  - Dependencies: T004-T007 must fail first

- [ ] **T010** [P] Integration test: Resource validation in `tests/integration/validation_test.go`:
  - Setup: Virtual node with known capacity
  - Test: Create pod requesting excessive CPU/memory
  - Assert: Pod fails with "Insufficient resources" error
  - Assert: Error message shows requested vs available
  - Reference: quickstart.md Scenario 3, research.md Section 4
  - Dependencies: T004-T007 must fail first

- [ ] **T011** [P] Integration test: Workload update (simple replace) in `tests/integration/update_test.go`:
  - Setup: Running pod on edge device
  - Test: Update pod spec (change image)
  - Assert: Old workload terminated before new starts (no overlap)
  - Assert: Update completes within 15s
  - Reference: quickstart.md Scenario 4, research.md Section 3
  - Dependencies: T004-T007 must fail first

- [ ] **T012** [P] Integration test: Device disconnection and timeout in `tests/integration/failover_test.go`:
  - Setup: Running pod on device-1
  - Test: Simulate device disconnection
  - Assert: Pod status changes to Unknown
  - Assert: After timeout (5min), pod deleted and rescheduled
  - Test: Reconnect before timeout
  - Assert: Pod status restored, no rescheduling
  - Reference: quickstart.md Scenario 5, research.md Section 5
  - Dependencies: T004-T007 must fail first

- [ ] **T013** [P] Integration test: Pod deletion and cleanup in `tests/integration/cleanup_test.go`:
  - Setup: Running pod on edge device
  - Test: Delete pod via kubectl
  - Assert: Workload removed from mock Flightctl within 10s
  - Assert: Node allocatable resources updated
  - Reference: quickstart.md Scenario 6
  - Dependencies: T004-T007 must fail first

---

## Phase 3.3: Core Implementation (ONLY after tests T004-T013 are failing)

### Data Models

- [x] **T014** [P] Implement Device model in `pkg/models/device.go`:
  - Define Device struct with ID, Name, FleetID, Labels, Capacity, Allocatable, Status, ConnectionState
  - Define DeviceStatus, DevicePhase enums
  - Implement state transitions (Ready ↔ NotReady ↔ Unknown)
  - Add validation methods (capacity > 0, allocatable <= capacity)
  - Reference: data-model.md Section 1
  - Dependencies: T004-T007 tests must be failing

- [x] **T015** [P] Implement Fleet model in `pkg/models/fleet.go`:
  - Define Fleet struct with ID, Name, Labels, DeviceCount
  - Add validation (DeviceCount >= 0)
  - Reference: data-model.md Section 2
  - Dependencies: T004-T007 tests must be failing

- [x] **T016** [P] Implement PodDeviceMapping in `pkg/models/pod_mapping.go`:
  - Define PodDeviceMapping struct with PodKey, PodUID, DeviceID, DeployedAt
  - No complex state machine - simple tracking index
  - Reference: data-model.md Section 3 (Workload entity REMOVED per architectural simplification)
  - Dependencies: T004-T007 tests must be failing
  - **Rationale**: Constitution Principle VII (Simplicity) - eliminated Workload abstraction, work directly with v1.Pod

- [x] **T017** [P] Implement DeploymentTarget model in `pkg/models/target.go`:
  - Define DeploymentTarget struct with FleetID, Selectors, DeviceID
  - Implement device selection algorithm:
    1. Build candidate list (fleet + label filters)
    2. Filter by ConnectionState=Connected and sufficient resources
    3. Sort by available resources, tie-break by workload count
  - Add validation (at least one targeting field set)
  - Reference: data-model.md Section 4
  - Dependencies: T014 (Device model)

- [x] **T018** [P] Implement DeviceStatusSnapshot in `pkg/models/snapshot.go`:
  - Define DeviceStatusSnapshot struct with DeviceID, Timestamp, Status, ConnectionState, Allocatable, RunningPods
  - Implement 30-second TTL caching logic
  - Add PodSummary struct (Namespace, Name, UID, Phase, Resources)
  - Reference: data-model.md Section 5
  - Dependencies: T014

- [x] **T019** [P] Implement ReconciliationRecord in `pkg/models/reconcile.go`:
  - Define ReconciliationRecord struct with Timestamp, PodKey, Operation, DesiredState (v1.PodPhase), ActualState (v1.PodPhase), Action, Result
  - Define enums: ReconcileOperation, ReconcileAction, ReconcileResult
  - Implement circular buffer (last 100 records)
  - Reference: data-model.md Section 6
  - Dependencies: None (pure data structure)

- [x] **T020** [P] Implement TimeoutTracker in `pkg/models/timeout.go`:
  - Define TimeoutTracker struct with DeviceID, DisconnectedAt, TimeoutDuration, TimeoutAt, AffectedPods (pod keys), TimerCancelFunc
  - Implement timer creation with context.CancelFunc
  - Add validation (timeout >= 1min, <= 30min)
  - Reference: data-model.md Section 7, research.md Section 5
  - Dependencies: None

### Flightctl Client Implementation

- [x] **T021** Implement Flightctl HTTP client in `pkg/flightctl/client.go`:
  - Define FlightctlClient struct with HTTP client, API URL, auth token
  - Implement NewFlightctlClient factory with config validation
  - Implement Ping method (health check)
  - Add retry logic with exponential backoff
  - Add request/response logging
  - Reference: contracts/flightctl-client.go, research.md Section 2
  - Dependencies: T006-T007 contract tests

- [ ] **T022** Implement Flightctl DeviceManager in `pkg/flightctl/devices.go`:
  - Implement ListDevices with fleet and label filters
  - Implement GetDevice by ID
  - Map Flightctl API responses to Device model
  - Handle errors: ErrNotFound, ErrDeviceOffline, ErrUnauthorized
  - Reference: contracts/flightctl-client.go, data-model.md Section 1
  - Dependencies: T021 (client), T014 (Device model), T006 contract test

- [ ] **T023** [P] Implement Flightctl FleetManager in `pkg/flightctl/fleets.go`:
  - Implement ListFleets
  - Implement GetFleet by ID
  - Map Flightctl API responses to Fleet model
  - Reference: contracts/flightctl-client.go, data-model.md Section 2
  - Dependencies: T021 (client), T015 (Fleet model)

- [x] **T024** Implement Flightctl PodManager in `pkg/flightctl/pods.go`:
  - Implement DeployPod (convert v1.Pod → Flightctl workload format)
  - Implement UpdatePod (simple replace: delete old, deploy new)
  - Implement DeletePod (idempotent)
  - Implement GetPodStatus and ListPods
  - Map Flightctl responses to v1.PodStatus (no intermediate WorkloadStatus)
  - Reference: contracts/flightctl-client.go, data-model.md Section 3, research.md Section 2
  - Dependencies: T021 (client), T007 contract test
  - **Note**: No T016 (Workload model) dependency - work directly with v1.Pod

### Virtual Kubelet Provider Implementation

- [x] **T025** Implement PodLifecycleHandler.CreatePod in `pkg/provider/pods.go`:
  - Extract DeploymentTarget from pod.spec (nodeSelector, affinity)
  - Call device selection algorithm (T017)
  - Validate device has sufficient CPU/memory resources
  - Pass v1.Pod directly to FlightctlClient.DeployPod (no conversion to intermediate Workload)
  - Store PodDeviceMapping in cache (T016)
  - Update pod status to Pending → Running based on Flightctl response
  - Handle errors: no matching device, insufficient resources, API failures
  - Reference: contracts/virtual-kubelet-provider.go, research.md Sections 4, 6
  - Dependencies: T016 (PodDeviceMapping), T017 (DeploymentTarget), T024 (PodManager), T004 contract test

- [ ] **T026** Implement PodLifecycleHandler.UpdatePod in `pkg/provider/pods.go`:
  - Lookup device via PodDeviceMapping
  - Pass updated v1.Pod to FlightctlClient.UpdatePod (simple replace)
  - Update pod status during transition (Terminating → Pending → Running)
  - Reference: contracts/virtual-kubelet-provider.go, research.md Section 3
  - Dependencies: T016 (PodDeviceMapping), T024 (PodManager), T004 contract test
  - Note: Modifies same file as T025 (sequential, no [P])

- [ ] **T027** Implement PodLifecycleHandler DeletePod, GetPod, GetPods in `pkg/provider/pods.go`:
  - DeletePod: Call FlightctlClient.DeletePod, remove PodDeviceMapping, update node allocatable
  - GetPod: Query Flightctl via PodManager, map to v1.PodStatus
  - GetPods: Return all tracked pods from PodDeviceMapping index
  - Reference: contracts/virtual-kubelet-provider.go
  - Dependencies: T016 (PodDeviceMapping), T018 (DeviceStatusSnapshot), T024 (PodManager), T004 contract test
  - Note: Modifies same file as T025-T026 (sequential)

- [x] **T028** [P] Implement NodeProvider interface in `pkg/provider/node.go`:
  - Implement Ping (check Flightctl connectivity)
  - Implement GetNode (return virtual node with aggregated capacity from devices)
  - Implement NotifyNodeStatus (register callback for capacity changes)
  - Calculate node capacity: sum of all connected devices in fleet
  - Calculate node allocatable: capacity - allocated workloads
  - Reference: contracts/virtual-kubelet-provider.go, research.md Section 1
  - Dependencies: T014 (Device model), T022 (DeviceManager), T005 contract test

- [ ] **T029** Implement provider initialization in `pkg/provider/provider.go`:
  - Define Provider struct with FlightctlClient, NodeProvider, PodLifecycleHandler, caches
  - Implement NewProvider factory
  - Load configuration from env vars (FLIGHTCTL_API_URL, AUTH_TOKEN, FLEET_ID, etc.)
  - Initialize device metadata cache (refresh every 5 min)
  - Start node status update goroutine
  - Reference: contracts/virtual-kubelet-provider.go, research.md Sections 1, 9
  - Dependencies: T021 (FlightctlClient), T025-T028 (provider methods)

### Reconciliation Logic

- [ ] **T030** Implement pod reconciliation loop in `pkg/reconciler/pods.go`:
  - Create workqueue for pod events
  - Implement reconcile function:
    1. Fetch desired state (v1.Pod spec from Kubernetes)
    2. Fetch actual state (pod status from Flightctl via PodManager)
    3. Diff phases (desired vs actual)
    4. Execute action (Deploy, Replace, Remove, or UpdateStatus)
  - Poll Flightctl every 30s for status updates
  - Use informer resync (5 min) for full reconciliation
  - Log ReconciliationRecord for each iteration
  - Reference: research.md Section 3, data-model.md Section 6
  - Dependencies: T016 (PodDeviceMapping), T018 (DeviceStatusSnapshot), T019 (ReconciliationRecord), T024 (PodManager)

- [ ] **T031** [P] Implement resource validation in `pkg/reconciler/resource.go`:
  - Extract CPU/memory requests from pod.spec.containers[*].resources.requests
  - Query DeviceStatusSnapshot for target device capacity and allocations
  - Calculate: requests > allocatable → reject
  - Return clear error: "Device {id} has {available} CPU, requested {requested} CPU"
  - Reference: research.md Section 4, data-model.md Section 1
  - Dependencies: T014 (Device model), T018 (DeviceStatusSnapshot)

- [ ] **T032** [P] Implement timeout tracking in `pkg/reconciler/timeout.go`:
  - Detect device ConnectionState change to Disconnected
  - Create TimeoutTracker with configurable duration (env var DEVICE_RECONNECT_TIMEOUT, default 5m)
  - Start timer using time.AfterFunc
  - On timeout: Delete affected pods (triggers Kubernetes rescheduling)
  - On reconnect (before timeout): Cancel timer, restore pod status
  - Emit Kubernetes events: "DeviceDisconnected", "DeviceReconnected", "DeviceTimeoutExceeded"
  - Reference: research.md Section 5, data-model.md Section 7
  - Dependencies: T016 (PodDeviceMapping for affected pod lookup), T020 (TimeoutTracker model), T024 (PodManager for status queries)

---

## Phase 3.4: Integration

- [ ] **T033** Create RBAC manifests in `config/rbac.yaml`:
  - Define ServiceAccount: vk-flightctl-provider
  - Define ClusterRole with permissions:
    - pods: get, list, watch, update, patch, delete
    - pods/status: update, patch
    - nodes: get, list, watch, create, update, patch
    - nodes/status: update, patch
  - Define ClusterRoleBinding
  - Reference: research.md Section 7, quickstart.md Setup step 2
  - Dependencies: None

- [ ] **T034** Create provider deployment manifest in `config/deployment.yaml`:
  - Define Deployment with 1 replica
  - Container: vk-flightctl-provider image
  - Environment variables:
    - FLIGHTCTL_API_URL (from secret)
    - FLIGHTCTL_AUTH_TOKEN (from secret)
    - DEVICE_RECONNECT_TIMEOUT (from configmap, default 5m)
    - STATUS_POLL_INTERVAL (from configmap, default 30s)
    - FLEET_ID (optional, from configmap)
  - Mount secret: flightctl-credentials
  - ServiceAccount: vk-flightctl-provider
  - Resource requests: 2 CPU, 4Gi memory
  - Liveness probe: /healthz
  - Readiness probe: /healthz
  - Reference: research.md Section 10, quickstart.md Setup step 3
  - Dependencies: T033 (RBAC)

- [x] **T035** Implement main entrypoint in `cmd/vk-flightctl-provider/main.go`:
  - Load configuration from environment variables
  - Initialize FlightctlClient (T021)
  - Initialize Provider (T029)
  - Register provider with Virtual Kubelet framework
  - Start reconciliation loop (T030)
  - Start HTTP server for /healthz and /metrics endpoints
  - Handle graceful shutdown (SIGTERM, SIGINT)
  - Reference: research.md Sections 1, 9, 10
  - Dependencies: T021, T029, T030, T033-T034

### Observability

- [ ] **T036** [P] Implement structured logging in `pkg/provider/logging.go`:
  - Use logr interface (controller-runtime logger)
  - Define standard log fields: pod, device, fleet, operation, error
  - Log levels: Info (normal ops), Error (failures), Debug (detailed reconciliation)
  - Add logging to all provider methods and reconciler
  - Reference: research.md Section 9
  - Dependencies: None (can be done in parallel, imported by other files)

- [ ] **T037** [P] Implement Prometheus metrics in `pkg/provider/metrics.go`:
  - Define metrics (from research.md Section 9):
    - vk_flightctl_pods_total{status}
    - vk_flightctl_pod_deployment_duration_seconds{device}
    - vk_flightctl_devices_total{fleet, status}
    - vk_flightctl_device_capacity{fleet, device, resource}
    - vk_flightctl_reconcile_duration_seconds{operation}
    - vk_flightctl_device_disconnections_total{fleet, device}
  - Export metrics on /metrics endpoint
  - Instrument provider methods and reconciler with metric calls
  - Reference: research.md Section 9, contracts/virtual-kubelet-provider.go ProviderMetrics
  - Dependencies: None (can be done in parallel)

---

## Phase 3.5: Polish

### Unit Tests

- [ ] **T038** [P] Unit tests for reconciler logic in `tests/unit/reconciler_test.go`:
  - Test state diffing (desired vs actual)
  - Test action selection (Deploy, Replace, Remove, UpdateStatus)
  - Test edge cases: pod deleted during reconciliation, device disconnected mid-deploy
  - Use table-driven tests
  - Reference: research.md Section 8
  - Dependencies: T030 (reconciler implementation)

- [ ] **T039** [P] Unit tests for resource validation in `tests/unit/validation_test.go`:
  - Test CPU validation (requested vs available)
  - Test memory validation
  - Test error messages format
  - Test overcommit scenarios
  - Reference: research.md Section 4
  - Dependencies: T031 (resource validation implementation)

### Validation & Documentation

- [ ] **T040** Run quickstart validation scenarios:
  - Follow quickstart.md step-by-step
  - Deploy kind cluster, mock Flightctl server, provider
  - Execute all 6 test scenarios
  - Verify all success criteria pass
  - Document any issues found and fix
  - Update quickstart.md if steps need clarification
  - Reference: quickstart.md entire document
  - Dependencies: T001-T037 (all core implementation complete), T008-T013 (integration tests passing)

---

## Dependencies

**Critical Path** (longest dependency chain):
```
T001 (setup) → T002 (deps) → T004-T007 (contract tests) → T014-T020 (models) →
T021 (Flightctl client) → T024 (PodManager) → T016 (PodDeviceMapping) → T025 (CreatePod) →
T026-T027 (other pod methods) → T029 (provider init) → T030 (reconciler) →
T035 (main) → T040 (validation)
```

**Architectural Simplification** (Constitution Principle VII):
- Removed Workload abstraction layer - provider works directly with v1.Pod objects
- T016 changed from complex Workload model to simple PodDeviceMapping index
- T024 changed from WorkloadManager to PodManager (direct pod pass-through to Flightctl)
- Reduced complexity, eliminated duplicate state management

**Dependency Graph**:
- **Setup** (T001-T003): No dependencies
- **Contract Tests** (T004-T007): Require T002 (dependencies installed)
- **Integration Tests** (T008-T013): Require T004-T007 to be failing first (TDD)
- **Models** (T014-T020): Require T004-T007 tests failing, can run in parallel [P]
- **Flightctl Client**:
  - T021 (client base): Requires T014-T015 (Device, Fleet models), T006-T007 (contract tests)
  - T022 (devices): Requires T021, T014
  - T023 (fleets): Requires T021, T015 (parallel with T022)
  - T024 (pods): Requires T021, T007 (no T016 dependency - works directly with v1.Pod)
- **Provider**:
  - T025-T027 (pod lifecycle): Require T016 (PodDeviceMapping), T017, T024, T004 (sequential - same file)
  - T028 (node): Requires T014, T022, T005 (parallel with T025-T027)
  - T029 (provider init): Requires T021, T025-T028
- **Reconciler**:
  - T030 (pod reconcile): Requires T016 (PodDeviceMapping), T018, T019, T024
  - T031 (resource validation): Requires T014, T018 (parallel with T030)
  - T032 (timeout): Requires T016 (PodDeviceMapping), T020, T024 (parallel with T030-T031)
- **Integration** (T033-T035): T033 no deps, T034 requires T033, T035 requires T021, T029-T030, T033-T034
- **Observability** (T036-T037): No dependencies (parallel)
- **Polish** (T038-T040): Require corresponding implementations complete

---

## Parallel Execution Examples

### Phase 1: Contract Tests (T004-T007 in parallel)
```bash
# All contract tests can run simultaneously (different files, no shared state)
# Launch using Task agent or parallel make targets

Task "Contract test for Virtual Kubelet PodLifecycleHandler in tests/unit/provider_contract_test.go"
Task "Contract test for Virtual Kubelet NodeProvider in tests/unit/node_contract_test.go"
Task "Contract test for Flightctl DeviceManager in tests/unit/flightctl_device_contract_test.go"
Task "Contract test for Flightctl WorkloadManager in tests/unit/flightctl_workload_contract_test.go"

# OR using make:
make -j4 test-contract
```

### Phase 2: Integration Tests (T008-T013 in parallel)
```bash
# All integration tests use separate kind clusters or test namespaces

Task "Integration test: Deploy workload in tests/integration/deployment_test.go"
Task "Integration test: Fleet targeting in tests/integration/fleet_test.go"
Task "Integration test: Resource validation in tests/integration/validation_test.go"
Task "Integration test: Workload update in tests/integration/update_test.go"
Task "Integration test: Device disconnection in tests/integration/failover_test.go"
Task "Integration test: Pod deletion in tests/integration/cleanup_test.go"

# OR using go test with parallel execution:
go test -v -parallel 6 ./tests/integration/...
```

### Phase 3: Model Implementation (T014-T020 in parallel)
```bash
# All model files are independent (different files, no imports between them)

Task "Implement Device model in pkg/models/device.go"
Task "Implement Fleet model in pkg/models/fleet.go"
Task "Implement PodDeviceMapping in pkg/models/pod_mapping.go"  # simplified from Workload
Task "Implement DeploymentTarget in pkg/models/target.go"  # depends on T014
Task "Implement DeviceStatusSnapshot in pkg/models/snapshot.go"  # depends on T014
Task "Implement ReconciliationRecord in pkg/models/reconcile.go"
Task "Implement TimeoutTracker in pkg/models/timeout.go"

# Note: T017, T018 have dependencies, so stagger execution:
# 1. Launch T014-T016, T019-T020 together (5 tasks)
# 2. When T014 complete, launch T017
# 3. When T014 complete, launch T018 (no longer depends on T016 - removed Workload)
```

### Phase 4: Flightctl Client (T022-T024 in parallel, after T021)
```bash
# After T021 (client base) completes:

Task "Implement Flightctl DeviceManager in pkg/flightctl/devices.go"
Task "Implement Flightctl FleetManager in pkg/flightctl/fleets.go"
Task "Implement Flightctl PodManager in pkg/flightctl/pods.go"  # no model dependency - works with v1.Pod directly
```

### Phase 5: Reconciler (T030-T032 in parallel)
```bash
# All reconciler files are independent

Task "Implement pod reconciliation in pkg/reconciler/pods.go"  # renamed from workload.go
Task "Implement resource validation in pkg/reconciler/resource.go"
Task "Implement timeout tracking in pkg/reconciler/timeout.go"
```

### Phase 6: Observability (T036-T037 in parallel)
```bash
# Logging and metrics are independent

Task "Implement structured logging in pkg/provider/logging.go"
Task "Implement Prometheus metrics in pkg/provider/metrics.go"
```

### Phase 7: Polish Unit Tests (T038-T039 in parallel)
```bash
# Different test files, no shared state

Task "Unit tests for reconciler in tests/unit/reconciler_test.go"
Task "Unit tests for validation in tests/unit/validation_test.go"
```

---

## Notes

- **[P] tasks** = Different files, no dependencies → can run in parallel
- **Sequential tasks** (no [P]) = Same file or direct dependency → must run in order
- **TDD enforcement**: T004-T013 (tests) MUST be written and failing BEFORE T014-T037 (implementation)
- **Commit strategy**: Commit after each task or logical group (e.g., all models T014-T020)
- **Integration tests**: Require kind cluster - use `kind create cluster --name vk-test` before T008-T013
- **Mock Flightctl**: Integration tests use httptest mock server, not real Flightctl instance

---

## Validation Checklist
*GATE: Verify before marking tasks.md complete*

- [x] All contracts have corresponding tests (T004-T007)
- [x] All entities have model tasks (T014-T020 for 6 entities in data-model.md: Device, Fleet, PodDeviceMapping, DeploymentTarget, DeviceStatusSnapshot, ReconciliationRecord, TimeoutTracker)
- [x] All tests come before implementation (T004-T013 before T014-T037)
- [x] Parallel tasks truly independent (all [P] tasks use different files)
- [x] Each task specifies exact file path (all tasks have explicit paths)
- [x] No task modifies same file as another [P] task (verified: T025-T027 sequential in pods.go, no other conflicts)
- [x] All quickstart scenarios covered (6 scenarios → T008-T013)
- [x] Observability included (T036-T037 for logging + metrics)
- [x] RBAC and deployment manifests included (T033-T034)
- [x] Main entrypoint defined (T035)

---

## Summary

**Total Tasks**: 40
- Setup: 3 tasks (T001-T003)
- Tests First (TDD): 10 tasks (T004-T013, must fail before implementation)
- Core Implementation: 24 tasks (T014-T037)
- Polish: 3 tasks (T038-T040)

**Parallel Tasks**: 27 tasks marked [P]
**Sequential Tasks**: 13 tasks (same file or direct dependencies)

**Estimated Timeline** (assuming 1 developer, 8hr workdays):
- Phase 3.1 Setup: 0.5 days (T001-T003)
- Phase 3.2 Tests: 2 days (T004-T013, TDD - write failing tests)
- Phase 3.3 Core: 6 days (T014-T037, implement to pass tests)
- Phase 3.4 Integration: 1 day (T033-T035, manifests + main)
- Phase 3.5 Polish: 1 day (T038-T040, unit tests + validation)
- **Total: ~10.5 days** (can be reduced with parallel execution and multiple developers)

**Next Command**: Begin with T001 or use Task agent to parallelize T004-T007 contract tests after setup.
