<!--
Sync Impact Report (2025-10-07):
- Version change: [UNPOPULATED] → 1.0.0 (Initial ratification)
- Modified principles: N/A (initial creation)
- Added sections: All core principles (7), Kubernetes Operator Standards, Quality Gates, Governance
- Removed sections: N/A
- Templates requiring updates:
  ✅ plan-template.md - Constitution Check section references applied
  ✅ spec-template.md - NFR requirements aligned
  ✅ tasks-template.md - TDD enforcement aligned
- Follow-up TODOs: None (all placeholders populated)

Rationale for version 1.0.0:
- First populated constitution for this project
- Establishes baseline governance for Kubernetes operator development
- Incorporates industry-standard practices for Virtual Kubelet providers
-->

# VK-Flightctl Provider Constitution

## Core Principles

### I. Single Responsibility
Each component serves ONE well-defined purpose within the Kubernetes operator pattern. The Virtual Kubelet provider manages workload lifecycle on Flightctl-managed edge devices—nothing more. No feature creep: if functionality doesn't directly support pod scheduling, workload reconciliation, or node status reporting, it belongs in a separate component or is out of scope.

### II. Kubernetes API Compliance (NON-NEGOTIABLE)
All implementations MUST adhere to Kubernetes API contracts and conventions:
- Virtual Kubelet PodLifecycleHandler and NodeProvider interfaces are sacred
- Standard Kubernetes resource semantics (pods, nodes, events) apply without deviation
- RBAC enforcement uses native Kubernetes mechanisms—no custom auth layers
- Status reporting follows Kubernetes phase/condition patterns exactly

Violations of K8s API contracts are automatic blockers. When in doubt, reference upstream Kubernetes controller-runtime and Virtual Kubelet SDK examples.

### III. Test-First Development (NON-NEGOTIABLE)
TDD cycle strictly enforced:
1. Write contract/integration tests that define expected behavior
2. Verify tests FAIL (red state)
3. Get explicit user/reviewer approval on test coverage
4. Implement minimum code to make tests PASS (green state)
5. Refactor with tests as safety net

Contract tests (T004-T007) MUST exist and fail before ANY implementation tasks (T014+) begin. Integration tests (T008-T013) validate user scenarios from quickstart.md. No exceptions.

### IV. Declarative Reconciliation
All edge device workload management follows declarative principles:
- Desired state defined in Kubernetes (pod specs, node selectors)
- Actual state queried from Flightctl devices
- Reconciliation loop continuously converges actual → desired
- Operations MUST be idempotent (applying same desired state multiple times = same result)
- Eventual consistency acceptable; manual intervention to fix state is not

Imperative commands (e.g., "start workload X on device Y now") violate this principle. The reconciler decides when/how/where based on declarative inputs.

### V. Observability & Debuggability
Every operation MUST produce structured, actionable telemetry:
- **Logging**: Use logr interface with consistent fields (pod, device, fleet, operation, error)
  - Info: Normal operations (reconciliation start/complete, workload deployed)
  - Error: Failures requiring attention (API errors, resource validation failures)
  - Debug: Detailed state transitions (useful for troubleshooting, disabled in production)
- **Metrics**: Prometheus-compatible counters, histograms, gauges (see research.md Section 9)
  - vk_flightctl_pods_total{status}, vk_flightctl_pod_deployment_duration_seconds, etc.
- **Events**: Emit Kubernetes events for user-visible state changes (DeviceDisconnected, WorkloadDeployed)

Text I/O debugging: All Flightctl API interactions log request/response (sanitize auth tokens). Operator state inspectable via kubectl describe, logs, and metrics endpoints.

### VI. Failure Tolerance & Graceful Degradation
Edge devices WILL disconnect, APIs WILL timeout, resources WILL be insufficient—design for these realities:
- Device disconnection triggers timeout-based rescheduling (FR-010, FR-010a)
- Flightctl API errors trigger exponential backoff retries, not crash loops
- Resource validation rejects workloads BEFORE attempting deployment (fail-fast with clear errors)
- Operator restart MUST NOT lose critical state (reconciliation resumes from Kubernetes/Flightctl state)
- Status polling failures degrade gracefully (mark device Unknown, continue monitoring other devices)

Crash loops and silent failures violate this principle. Every error path has an explicit recovery strategy.

### VII. Simplicity & Minimalism (YAGNI)
Start with the simplest solution that meets requirements:
- Simple replace-only update strategy (stop old, start new)—no rolling updates, canary deployments, or complex orchestration until proven necessary
- CPU/memory resource validation only—no GPU, storage, or custom resource checks until required
- 30-second status polling interval—no complex event-driven watch mechanisms until scale demands it

Complexity MUST be justified in plan.md Complexity Tracking section. Before adding abstraction layers (e.g., Repository pattern, event bus, plugin system), demonstrate why direct implementation is insufficient.

## Kubernetes Operator Standards

### Architecture Requirements
- **Controller Pattern**: Use controller-runtime workqueue and informer patterns for pod event handling
- **Leader Election**: Not required for single-replica deployment (v1 scope: 1 provider instance)
- **CRDs**: Not required—operate on standard Kubernetes pod/node resources only
- **Admission Webhooks**: Not required for v1 (validation happens in CreatePod handler)

### Integration Requirements
- **Flightctl Client**: Wrap Flightctl REST API with typed Go client (pkg/flightctl/)
- **Virtual Kubelet SDK**: Implement provider.Provider interface per upstream conventions
- **Kubernetes Client**: Use client-go and controller-runtime for pod/node operations
- **Authentication**: Flightctl API token via Secret, Kubernetes RBAC via ServiceAccount

### Performance & Scale Targets
- **Devices per fleet**: 100+ (v1 target: 500 devices across 10 fleets)
- **Workload deployment latency**: <5 seconds (pod create → Running status)
- **Reconciliation interval**: 30 seconds (status polling) + 5 minutes (full resync)
- **Resource footprint**: Provider pod ≤2 CPU, ≤4Gi memory under normal load

Failure to meet these targets in validation (T040) blocks release.

## Quality Gates

### Pre-Implementation Gates
1. **Constitution Check** (plan.md): All principles evaluated, violations justified or design refactored
2. **Clarification Resolution** (spec.md): No [NEEDS CLARIFICATION] markers remain OR explicitly deferred with justification
3. **Contract Tests Written** (T004-T007): All provider/client interfaces have failing tests

### Pre-Merge Gates
1. **All Tests Pass**: Contract tests (T004-T007), integration tests (T008-T013), unit tests (T038-T039)
2. **Quickstart Validation** (T040): All 6 user scenarios succeed end-to-end
3. **Linting Clean**: golangci-lint passes with zero warnings
4. **Performance Verified**: Deployment latency <5s, supports 100+ devices (measured in T040)
5. **Observability Functional**: Logs structured, metrics exported, events emitted

### Pre-Release Gates
1. **Security Review**: RBAC permissions minimal (least privilege), secrets handling validated
2. **Documentation Complete**: README usage instructions, quickstart.md executable, API contracts published
3. **Backwards Compatibility**: If amending existing APIs, migration path documented

## Governance

### Amendment Process
1. Propose constitution change with rationale (GitHub issue/PR or equivalent)
2. Identify impact on existing code and design artifacts
3. Get approval from project maintainers/stakeholders
4. Update constitution version (MAJOR/MINOR/PATCH per semantic versioning)
5. Propagate changes to dependent templates (plan, spec, tasks)
6. Notify team via commit message referencing amendment

### Versioning Policy
- **MAJOR** (X.0.0): Backward-incompatible principle changes (e.g., removing TDD requirement, changing K8s API compliance rules)
- **MINOR** (x.Y.0): New principle additions, material expansions (e.g., adding security scanning requirement)
- **PATCH** (x.y.Z): Clarifications, typo fixes, non-semantic wording improvements

### Compliance Review
- **Every PR/feature**: Author self-certifies compliance via plan.md Constitution Check section
- **Pre-merge review**: Reviewer verifies no unjustified violations, complexity justified
- **Quarterly audit**: Review all Complexity Tracking entries—justify retention or refactor

### Conflict Resolution
- Constitution supersedes all other documentation (README, comments, tribal knowledge)
- When constitution conflicts with external standards (e.g., Kubernetes conventions), external standard wins—amend constitution to align
- For ambiguity: Interpret toward simplicity, testability, and declarative principles

**Version**: 1.0.0 | **Ratified**: 2025-10-07 | **Last Amended**: 2025-10-07
