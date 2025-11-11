# VK-Flightctl Provider

A Virtual Kubelet provider for deploying Kubernetes workloads to edge devices managed by Flightctl.

## Overview

This provider implements the Virtual Kubelet interface to enable Kubernetes workloads to run on edge devices managed by Flightctl, without requiring those devices to run a full Kubernetes node.

**Status**: Minimal Working Prototype ✅

### Architecture

- **Direct Pod Pass-through**: Works directly with Kubernetes `v1.Pod` objects (no intermediate Workload abstraction)
- **Simplified Design**: Per Constitution Principle VII (Simplicity & Minimalism)
- **Virtual Kubelet SDK**: Implements `PodLifecycleHandler` and `NodeProvider` interfaces
- **Flightctl Integration**: HTTP client for device and pod management

## Quick Start

### Prerequisites

- Go 1.21+
- Access to a Flightctl API instance
- Flightctl API authentication token

### Build

```bash
make build
# Binary created at: bin/vk-flightctl-provider
```

### Configuration

Set required environment variables:

```bash
export FLIGHTCTL_API_URL="https://flightctl.example.com"
export FLIGHTCTL_AUTH_TOKEN="your-api-token"
export NODE_NAME="vk-flightctl-node"  # optional, defaults to "vk-flightctl-node"
```

Optional configuration:

```bash
export FLIGHTCTL_INSECURE_TLS="true"  # Skip TLS verification (testing only)
```

### Run

```bash
./bin/vk-flightctl-provider
```

Expected output:
```
Starting VK-Flightctl Provider...
Successfully connected to Flightctl API
Virtual node 'vk-flightctl-node' initialized with capacity: CPU=4, Memory=8Gi
Provider is running. Press Ctrl+C to exit.
```

## Project Structure

```
├── cmd/vk-flightctl-provider/  # Main entrypoint
├── pkg/
│   ├── provider/               # Virtual Kubelet provider implementation
│   ├── flightctl/              # Flightctl API client
│   │   ├── client.go          # Base HTTP client
│   │   └── pods.go            # Pod management (direct v1.Pod handling)
│   └── models/                 # Data models
│       ├── device.go          # Edge device representation
│       ├── fleet.go           # Fleet grouping
│       ├── pod_mapping.go     # Simple pod-to-device index
│       ├── target.go          # Device selection logic
│       ├── snapshot.go        # Device status caching
│       ├── reconcile.go       # Reconciliation tracking
│       └── timeout.go         # Disconnection timeout handling
├── tests/
│   ├── unit/                   # Unit and contract tests
│   └── integration/            # Integration tests
└── config/                     # Kubernetes manifests
```

## Implementation Status

### ✅ Completed (Minimal Prototype)

- **T001-T003**: Project setup, dependencies, tooling
- **T014-T020**: Core data models (Device, Fleet, PodDeviceMapping, etc.)
- **T021**: Flightctl HTTP client
- **T024**: PodManager (direct pod handling, no Workload abstraction)
- **T025**: Provider CreatePod implementation
- **T028**: NodeProvider implementation
- **T035**: Main entrypoint

### 🚧 Not Yet Implemented (Full Production)

- **T004-T013**: Contract and integration tests (TDD)
- **T022-T023**: Device and Fleet managers
- **T026-T027**: UpdatePod, DeletePod, GetPod(s) full implementation
- **T029**: Provider initialization with caching
- **T030-T032**: Reconciliation loop, resource validation, timeout tracking
- **T033-T034**: RBAC manifests, deployment configuration
- **T036-T037**: Observability (logging, metrics)
- **T038-T040**: Unit tests, performance validation

## Key Design Decisions

### Architectural Simplification

**Removed Workload Abstraction** (Constitution Principle VII):
- Provider works directly with `v1.Pod` objects
- Pod specs passed directly to Flightctl HTTP client
- Flightctl status mapped directly to `v1.PodStatus`
- Eliminated duplicate state management

**Benefits**:
- ✅ Reduced complexity
- ✅ Fewer transformations
- ✅ Canonical state in Kubernetes
- ✅ Simpler testing

### Minimal Prototype Limitations

This prototype includes:
- ✅ Basic pod deployment to mock device
- ✅ Provider-Flightctl connectivity
- ✅ Virtual node representation
- ✅ Simple pod-to-device tracking

Not included yet:
- ❌ Device selection algorithm (uses mock device)
- ❌ Status reconciliation loop
- ❌ Resource validation
- ❌ Timeout handling for disconnected devices
- ❌ RBAC configuration
- ❌ Production-grade error handling
- ❌ Metrics and observability

## Development

### Linting

```bash
make lint
```

### Formatting

```bash
make fmt
```

### Testing

```bash
make test              # All tests
make test-contract     # Contract tests only
make test-integration  # Integration tests only
```

### Clean

```bash
make clean
```

## Constitution Compliance

This project follows the constitution defined in `.specify/memory/constitution.md` v1.0.0:

- **Principle I (Single Responsibility)**: ✅ Focused on workload lifecycle for Flightctl edge devices
- **Principle II (Kubernetes API Compliance)**: ✅ Implements Virtual Kubelet interfaces
- **Principle III (Test-First Development)**: 🚧 Tests pending (T004-T013)
- **Principle IV (Declarative Reconciliation)**: 🚧 Reconciliation loop pending (T030)
- **Principle V (Observability)**: 🚧 Logging/metrics pending (T036-T037)
- **Principle VI (Failure Tolerance)**: 🚧 Timeout handling pending (T032)
- **Principle VII (Simplicity & Minimalism)**: ✅ Workload abstraction removed

## Contributing

See `specs/001-we-want-to/tasks.md` for the complete implementation plan.

## License

[Add license information]

## References

- [Virtual Kubelet](https://virtual-kubelet.io/)
- [Flightctl](https://github.com/flightctl/flightctl)
- [Kubernetes API](https://kubernetes.io/docs/reference/kubernetes-api/)
- [Project Specification](specs/001-we-want-to/spec.md)
- [Technical Plan](specs/001-we-want-to/plan.md)
- [Data Model](specs/001-we-want-to/data-model.md)
