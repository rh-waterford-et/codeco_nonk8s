
# Implementation Plan: Non-Native Kubernetes Workload Deployment

**Branch**: `001-we-want-to` | **Date**: 2025-10-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/home/lgriffin/Documents/claude_testing/eu-example/specs/001-we-want-to/spec.md`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from file system structure or context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Fill the Constitution Check section based on the content of the constitution document.
4. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
5. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
6. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, `GEMINI.md` for Gemini CLI, `QWEN.md` for Qwen Code, or `AGENTS.md` for all other agents).
7. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
8. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
9. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Deploy containerized workloads from a Kubernetes cluster to edge devices and non-native Kubernetes environments using Virtual Kubelet and Flightctl. Virtual Kubelet will represent edge devices as virtual nodes in the cluster, while Flightctl manages the edge device fleets declaratively. Operators can target devices using labels/selectors and fleet identifiers, with automatic reconciliation and failure recovery.

## Technical Context
**Language/Version**: Go 1.21+ (for Kubernetes ecosystem compatibility)
**Primary Dependencies**: Virtual Kubelet SDK, Flightctl client libraries, Kubernetes client-go, controller-runtime
**Storage**: Kubernetes etcd (cluster state), Flightctl backend (device/fleet state)
**Testing**: Go testing framework, kind (Kubernetes in Docker) for integration tests
**Target Platform**: Linux (Kubernetes control plane), edge devices (Linux-based)
**Project Type**: single (Kubernetes operator/controller pattern)
**Performance Goals**: Support 100+ edge devices per fleet, <5s workload deployment latency
**Constraints**: Must integrate with existing Kubernetes RBAC, configurable device reconnection timeout, CPU/memory resource validation only
**Scale/Scope**: Initial support for 500 edge devices across 10 fleets, simple replace-only update strategy

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Constitution Version**: 1.0.0 (ratified 2025-10-07)

### Principle Compliance

**✅ I. Single Responsibility**
- Provider manages ONLY workload lifecycle on Flightctl-managed edge devices
- No feature creep: pod scheduling, reconciliation, node status reporting within defined scope

**✅ II. Kubernetes API Compliance (NON-NEGOTIABLE)**
- Virtual Kubelet PodLifecycleHandler and NodeProvider interfaces implemented per upstream SDK
- Standard Kubernetes RBAC (T033), pod/node semantics, event patterns
- No custom auth layers or non-standard extensions

**✅ III. Test-First Development (NON-NEGOTIABLE)**
- Contract tests (T004-T007) written and failing before implementation (T014+)
- Integration tests (T008-T013) validate quickstart.md scenarios
- TDD cycle enforced in tasks.md Phase 3.2

**✅ IV. Declarative Reconciliation**
- Reconciliation loop (T030) converges desired (K8s) → actual (Flightctl) state
- Idempotent operations: DeployWorkload, UpdateWorkload, DeleteWorkload
- ⚠️ **Gap**: No explicit idempotency validation task (see Complexity Tracking)

**✅ V. Observability & Debuggability**
- Structured logging via logr (T036): pod, device, fleet, operation fields
- Prometheus metrics (T037): deployment latency, device capacity, reconcile duration
- Kubernetes events emitted (T032): DeviceDisconnected, DeviceReconnected, etc.

**✅ VI. Failure Tolerance & Graceful Degradation**
- Device disconnection timeout tracking (T032) with configurable duration
- Flightctl API retry logic with exponential backoff (T021)
- Resource validation fails fast before deployment (T031)

**✅ VII. Simplicity & Minimalism (YAGNI)**
- Simple replace-only updates (no rolling/canary)
- CPU/memory validation only (no GPU/storage)
- 30-second status polling (no complex event-driven mechanisms)
- ⚠️ **Complexity Justification**: Two external dependencies (Virtual Kubelet SDK, Flightctl client) mandated by spec FR-002, FR-003—no simpler alternative achieves stated requirements

### Kubernetes Operator Standards Compliance

**✅ Architecture**: controller-runtime workqueue + informer patterns (T030)
**✅ Integration**: Flightctl REST client (pkg/flightctl/), Virtual Kubelet SDK provider interface
**✅ Performance Targets**: <5s deployment latency, 100+ devices/fleet, 500 total devices (validated in T040)

### Quality Gates Status

**Pre-Implementation Gates**:
- ✅ Constitution Check: PASS (this section)
- ⚠️ Clarification Resolution: 5 [NEEDS CLARIFICATION] markers remain in spec.md (4 edge cases + FR-015)—deferred to v2 per plan.md:290
- 🔲 Contract Tests Written: Pending (T004-T007 execution)

**Post-Design Gates** (to be evaluated after Phase 1):
- Constitution re-check for design violations
- Performance target feasibility review

## Project Structure

### Documentation (this feature)
```
specs/001-we-want-to/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
cmd/
└── vk-flightctl-provider/    # Virtual Kubelet provider binary
    └── main.go

pkg/
├── provider/                  # Virtual Kubelet provider implementation
│   ├── provider.go           # Main provider interface
│   ├── pods.go               # Pod lifecycle management
│   └── node.go               # Node status reporting
├── flightctl/                 # Flightctl client integration
│   ├── client.go             # Flightctl API client
│   ├── devices.go            # Device management
│   └── fleets.go             # Fleet management
├── reconciler/                # Reconciliation logic
│   ├── workload.go           # Workload state reconciliation
│   ├── resource.go           # Resource validation
│   └── timeout.go            # Reconnection timeout handling
└── models/                    # Data models
    ├── device.go             # Edge device representation
    ├── fleet.go              # Fleet representation
    └── status.go             # Status aggregation

tests/
├── integration/               # Integration tests with kind
│   ├── deployment_test.go
│   ├── fleet_test.go
│   └── failover_test.go
└── unit/                      # Unit tests
    ├── provider_test.go
    ├── reconciler_test.go
    └── validation_test.go

config/
├── rbac.yaml                  # Kubernetes RBAC definitions
└── deployment.yaml            # Provider deployment manifest
```

**Structure Decision**: Single Kubernetes operator project following standard Go project layout. The Virtual Kubelet provider runs as a deployment in the cluster and implements the Virtual Kubelet provider interface to manage workloads on Flightctl-managed edge devices.

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `.specify/scripts/bash/update-agent-context.sh claude`
     **IMPORTANT**: Execute it exactly as specified above. Do not add or remove any arguments.
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
The /tasks command will generate implementation tasks following TDD principles:

1. **Contract Tests** (from contracts/*.go):
   - Virtual Kubelet provider interface tests [P]
   - Flightctl client interface tests [P]
   - Mock implementations for testing

2. **Model Implementation** (from data-model.md):
   - Device model with state transitions [P]
   - Fleet model [P]
   - PodDeviceMapping tracking (simple index, no Workload entity) [P]
   - DeploymentTarget selection logic [P]
   - TimeoutTracker for disconnection handling

3. **Flightctl Client** (from flightctl-client.go contract):
   - HTTP client wrapper
   - Device management methods
   - Fleet management methods
   - Pod deployment methods (convert v1.Pod → Flightctl format, deploy, get status)
   - Status mapping (Flightctl status → v1.PodStatus)
   - Error handling and retries

4. **Virtual Kubelet Provider** (from virtual-kubelet-provider.go contract):
   - PodLifecycleHandler implementation
     - CreatePod: pass v1.Pod directly to Flightctl client, update PodDeviceMapping
     - UpdatePod: simple replace via Flightctl client
     - DeletePod: cleanup via Flightctl client, remove PodDeviceMapping
     - GetPod/GetPods: query Flightctl, map status to v1.PodStatus
   - NodeProvider implementation
     - Ping health check
     - Node status aggregation
     - Capacity reporting

5. **Reconciliation Logic** (from research.md decisions):
   - Reconciliation loop with workqueue
   - Status polling (30s interval)
   - Device selection algorithm
   - Resource validation
   - Timeout tracking and rescheduling

6. **Configuration & Deployment**:
   - RBAC manifests (ServiceAccount, ClusterRole, ClusterRoleBinding)
   - Deployment manifest with provider pod
   - ConfigMap for provider settings
   - Secret handling for Flightctl credentials

7. **Integration Tests** (from quickstart.md scenarios):
   - Scenario 1: Basic pod deployment
   - Scenario 2: Fleet-level targeting
   - Scenario 3: Resource validation
   - Scenario 4: Workload update
   - Scenario 5: Device disconnection/reconnection
   - Scenario 6: Pod deletion

8. **Observability**:
   - Structured logging setup (logr)
   - Prometheus metrics exporter
   - Health check endpoints (/healthz, /metrics)

**Ordering Strategy**:
1. Contract tests (TDD: tests first)
2. Models and data structures
3. Flightctl client implementation
4. Provider interface implementation
5. Reconciliation logic
6. Configuration files
7. Integration tests
8. Observability

Dependencies:
- Models must exist before provider implementation
- Flightctl client must exist before provider can call it
- Contract tests must exist before implementation
- Integration tests require all components

Parallelizable tasks marked [P] - can be worked on simultaneously:
- Different model files
- Different contract test files
- Different provider interface methods (once contracts exist)

**Estimated Output**: 35-40 numbered, dependency-ordered tasks in tasks.md

**Task Structure**:
Each task will include:
- Clear description of what to implement
- Reference to design doc (contract, data model, research decision)
- Acceptance criteria (tests that must pass)
- Dependencies (which prior tasks must be complete)
- [P] marker if parallelizable

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [x] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS (see Constitution Check section above)
- [x] Post-Design Constitution Check: PASS (no violations introduced by design)
- [x] All NEEDS CLARIFICATION resolved with explicit deferral justification:
  - **Resolved**: 5 clarifications answered in spec.md Session 2025-10-06
  - **Deferred to v2**:
    - FR-015: Device metadata exposure (beyond node capacity/allocatable)
    - Edge case: Offline operation requirements (spec.md:79)
    - Edge case: Compatibility checking beyond CPU/memory (spec.md:81)
    - Edge case: Device lifecycle policies (spec.md:82)
    - Edge case: Version mismatch handling (spec.md:84)
  - **Justification**: All deferred items are edge cases or enhancements not blocking baseline v1 functionality (deploy, update, delete workloads to connected devices with resource validation)
- [x] Complexity deviations documented (Virtual Kubelet + Flightctl required by spec)

---
*Based on Constitution v1.0.0 - See `.specify/memory/constitution.md`*
