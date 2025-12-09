# Pod Status Management

This document explains how pod status is tracked and updated in the Virtual Kubelet FlightCtl provider.

## Overview

The provider implements a **3-tier status management strategy** to efficiently track pod status while minimizing API calls to FlightCtl:

1. **Immediate status** - Set when pod is created
2. **Background reconciliation** - Periodic sync with FlightCtl
3. **Cached status** - Fast reads from in-memory cache

## Architecture

### Data Model

The `PodDeviceMapping` struct ([pkg/models/pod_mapping.go](../pkg/models/pod_mapping.go)) tracks the relationship between Kubernetes pods and FlightCtl devices:

```go
type PodDeviceMapping struct {
    PodKey     string            // "namespace/name"
    Namespace  string            // Pod namespace
    Name       string            // Pod name
    PodUID     types.UID         // Kubernetes pod UID
    DeviceID   string            // FlightCtl device ID
    DeployedAt time.Time         // Deployment timestamp
    Status     *corev1.PodStatus // Cached pod status (updated by reconciliation loop)
}
```

### Status Flow

```
┌─────────────┐
│ CreatePod() │
└──────┬──────┘
       │
       ├─► Deploy to FlightCtl Device
       │
       └─► Set initial Pending status ──┐
                                        │
       ┌────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────┐
│  Background Reconciliation Loop  │  ◄── Runs every 15 seconds
│  (syncPodStatusLoop)             │
└──────┬───────────────────────────┘
       │
       ├─► Query FlightCtl Device status
       │
       └─► Update cached status in PodDeviceMapping
                                        │
       ┌────────────────────────────────┘
       │
       ▼
┌──────────────┐
│  GetPod()    │  ──► Return cached status (fast)
└──────────────┘
```

## Implementation Details

### 1. Initial Status (CreatePod)

When a pod is created in [pkg/provider/provider.go:143](../pkg/provider/provider.go#L143):

```go
func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
    // ... deploy to FlightCtl ...

    // Set initial Pending status
    mapping.Status = &corev1.PodStatus{
        Phase: corev1.PodPending,
        Conditions: []corev1.PodCondition{
            {
                Type:               corev1.PodScheduled,
                Status:             corev1.ConditionTrue,
                LastTransitionTime: metav1.Now(),
                Reason:             "Scheduled",
                Message:            fmt.Sprintf("Pod scheduled to FlightCtl device %s", deviceID),
            },
        },
    }
}
```

This provides immediate feedback to Kubernetes that the pod has been scheduled.

### 2. Background Reconciliation

The provider starts a background goroutine in [NewProvider()](../pkg/provider/provider.go#L77) that runs every 15 seconds:

```go
func (p *Provider) syncPodStatusLoop() {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-p.reconcileCtx.Done():
            return
        case <-ticker.C:
            p.reconcilePodStatus()
        }
    }
}
```

The reconciliation process ([reconcilePodStatus](../pkg/provider/provider.go#L98)):
1. Takes a snapshot of all pod mappings (to avoid holding locks during API calls)
2. Queries FlightCtl for each pod's current status
3. Updates the cached status in the mapping

**Lock Management:**
- Uses `RLock` to read the mappings list (allows concurrent reads)
- Releases lock before making HTTP calls (prevents blocking)
- Uses `Lock` only when updating individual cached statuses

### 3. Cached Status Retrieval

When Kubernetes queries pod status via [GetPod()](../pkg/provider/provider.go#L223):

```go
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
    // ... get mapping ...

    // Use cached status if available
    if mapping.Status != nil {
        pod.Status = *mapping.Status
        return pod, nil
    }

    // Fallback: query FlightCtl if no cached status
    status, err := p.podManager.GetPodStatus(ctx, pod, mapping.DeviceID)
    // ... update cache and return ...
}
```

This provides fast reads from memory without hitting the FlightCtl API on every request.

### 4. FlightCtl Status Query

The [GetPodStatus()](../pkg/flightctl/pods.go#L107) method queries the FlightCtl Device resource and maps application status to pod status:

```go
func (pm *PodManager) GetPodStatus(ctx context.Context, pod *corev1.Pod, deviceID string) (*corev1.PodStatus, error) {
    appName := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)

    // Get the Device resource
    device, err := pm.getDevice(ctx, deviceID)

    // Check if application exists in device.spec.applications
    for _, app := range device.Spec.Applications {
        if app.Name == appName {
            appExists = true
            break
        }
    }

    // Check device.status.applications for actual runtime status
    if device.Status != nil && len(device.Status.Applications) > 0 {
        for _, appStatus := range device.Status.Applications {
            if appStatus.Name == appName {
                // Found runtime status - map to Kubernetes pod status
                return pm.mapFlightctlStatusToPodStatus(&appStatus), nil
            }
        }
    }

    // No runtime status yet - return Pending
    return &corev1.PodStatus{Phase: corev1.PodPending}, nil
}
```

**Status Mapping:** The implementation now properly maps FlightCtl application runtime status (`device.status.applications[].status`) to Kubernetes pod phases.

## Status Phases

### Kubernetes Pod Phases

| Phase | When Set | Meaning |
|-------|----------|---------|
| **Pending** | Immediately after CreatePod, or when app in spec but no runtime status | Pod scheduled to FlightCtl device, deployment in progress |
| **Running** | When FlightCtl reports status="running" | Application is running on the device |
| **Failed** | When FlightCtl reports status="failed" or "error" | Application deployment failed or runtime error |
| **Succeeded** | When FlightCtl reports status="completed", "succeeded", or "stopped" | Application completed successfully or stopped gracefully |

### FlightCtl to Kubernetes Status Mapping

The [mapFlightctlStatusToPodStatus()](../pkg/flightctl/pods.go#L154) method maps FlightCtl application statuses to Kubernetes pod phases:

| FlightCtl Status | Kubernetes Phase | Pod Condition | Reason |
|------------------|------------------|---------------|--------|
| `running` | Running | Ready=True | ApplicationRunning |
| `pending` | Pending | Scheduled=True | ApplicationStarting |
| `starting` | Pending | Scheduled=True | ApplicationStarting |
| `failed` | Failed | Ready=False | ApplicationFailed |
| `error` | Failed | Ready=False | ApplicationFailed |
| `completed` | Succeeded | Ready=False | ApplicationCompleted |
| `succeeded` | Succeeded | Ready=False | ApplicationCompleted |
| `stopped` | Succeeded | Ready=False | ApplicationStopped |
| *(unknown)* | Pending | Scheduled=True | UnknownStatus |
| *(no status)* | Pending | Scheduled=True | ApplicationDeployed |

**Note:** If the application exists in `device.spec.applications` but has no corresponding entry in `device.status.applications`, the pod is assumed to be Pending (waiting for the device to start the application).

## Graceful Shutdown

The provider supports graceful shutdown via the [Shutdown()](../pkg/provider/provider.go#L134) method:

```go
func (p *Provider) Shutdown() {
    if p.reconcileCancel != nil {
        p.reconcileCancel()  // Stops the background reconciliation loop
    }
}
```

## Performance Characteristics

### API Call Reduction

**Without caching (old approach):**
- Every GetPod() call → 1 HTTP GET to FlightCtl
- 100 GetPod() calls = 100 HTTP requests

**With caching (new approach):**
- Background reconciliation: 1 HTTP GET per pod every 15 seconds
- GetPod() calls → 0 HTTP requests (reads from cache)
- 100 GetPod() calls = 0 HTTP requests

### Trade-offs

| Aspect | Benefit | Cost |
|--------|---------|------|
| **Freshness** | Status updates every 15s | Up to 15s delay in status changes |
| **Performance** | Zero API calls for reads | Small memory overhead for cached status |
| **Concurrency** | Lock-free reads | Slightly more complex locking logic |

## Future Enhancements

1. **Event-driven updates**: Subscribe to FlightCtl events instead of polling
2. **Adaptive polling**: Increase frequency for pods in transitional states
3. **Status field inspection**: Parse `device.status.applications[].state` for actual runtime status
4. **Backoff on errors**: Exponential backoff if FlightCtl API is unavailable
5. **Metrics**: Track reconciliation loop performance and cache hit rates

## Testing

Run the tests to verify status management:

```bash
cd /home/raycarroll/Documents/Code/codeco-nonk8-1
go test -v ./pkg/provider
go test -v ./pkg/flightctl
```

## Related Files

- Pod status management: [pkg/provider/provider.go](../pkg/provider/provider.go)
- Pod-to-device mapping: [pkg/models/pod_mapping.go](../pkg/models/pod_mapping.go)
- FlightCtl status query: [pkg/flightctl/pods.go](../pkg/flightctl/pods.go)
