# FlightCtl Application Status Mapping

This document explains how FlightCtl application status from the Device resource is mapped to Kubernetes pod status.

## Problem Statement

The initial implementation only checked if an application existed in the Device spec (`device.spec.applications[]`) and assumed it was "Running" if present. This was insufficient because:

1. **No runtime status**: Presence in spec ≠ actually running
2. **No failure detection**: Failed applications appeared as Running
3. **No pending state**: Starting applications appeared as Running immediately

## Solution

The implementation now queries the **Device status** (`device.status.applications[]`) to get actual runtime status and maps it to appropriate Kubernetes pod phases.

## Architecture

### FlightCtl Device Structure

```json
{
  "apiVersion": "flightctl.io/v1alpha1",
  "kind": "Device",
  "metadata": {
    "name": "device-123"
  },
  "spec": {
    "applications": [
      {
        "name": "default-nginx-pod",
        "appType": "compose",
        "inline": [...]
      }
    ]
  },
  "status": {
    "applications": [
      {
        "name": "default-nginx-pod",
        "status": "running",
        "summary": "Application is healthy"
      }
    ]
  }
}
```

### Status Query Flow

```
GetPodStatus(pod, deviceID)
    ↓
1. GET /api/v1/devices/{deviceID}
    ↓
2. Check device.spec.applications[] for app existence
    ↓ (not found)
    └─→ Return error: "application not found"
    ↓ (found)
3. Check device.status.applications[] for runtime status
    ↓ (no status)
    └─→ Return Pending (application not started yet)
    ↓ (has status)
4. Map FlightCtl status to Kubernetes phase
    ↓
5. Return PodStatus with mapped phase and conditions
```

## Status Mapping

### FlightCtl Application Statuses

FlightCtl applications can report various statuses in `device.status.applications[].status`:

- `running` - Application is running normally
- `pending` - Application is scheduled but not yet started
- `starting` - Application is in the process of starting
- `failed` - Application failed to start or crashed
- `error` - Application encountered an error
- `completed` - Application completed successfully (for jobs)
- `succeeded` - Application succeeded (alternative to completed)
- `stopped` - Application was stopped gracefully

### Mapping Table

| FlightCtl Status | K8s Phase | Ready Condition | Reason | Use Case |
|------------------|-----------|-----------------|--------|----------|
| **running** | Running | True | ApplicationRunning | Normal running state |
| **pending** | Pending | - | ApplicationStarting | Scheduled, waiting to start |
| **starting** | Pending | - | ApplicationStarting | Container startup in progress |
| **failed** | Failed | False | ApplicationFailed | Startup failure or crash |
| **error** | Failed | False | ApplicationFailed | Runtime error |
| **completed** | Succeeded | False | ApplicationCompleted | Job completed successfully |
| **succeeded** | Succeeded | False | ApplicationCompleted | Task finished |
| **stopped** | Succeeded | False | ApplicationStopped | Graceful shutdown |
| *(unknown)* | Pending | - | UnknownStatus | Unrecognized status |
| *(no status)* | Pending | - | ApplicationDeployed | In spec, not yet reported |

### Special Cases

#### Case 1: Application in Spec, No Status

```go
// Application exists in device.spec.applications
// BUT no entry in device.status.applications
return &corev1.PodStatus{
    Phase: corev1.PodPending,
    Conditions: []corev1.PodCondition{{
        Type:   corev1.PodScheduled,
        Status: corev1.ConditionTrue,
        Reason: "ApplicationDeployed",
    }},
}
```

**Interpretation:** The application has been added to the Device spec, but the device agent hasn't started it yet or hasn't reported status yet.

#### Case 2: Unknown Status

```go
// device.status.applications[].status = "initializing" (unknown)
return &corev1.PodStatus{
    Phase: corev1.PodPending,
    Conditions: []corev1.PodCondition{{
        Type:    corev1.PodScheduled,
        Status:  corev1.ConditionTrue,
        Reason:  "UnknownStatus",
        Message: "Unknown application status: initializing - ...",
    }},
}
```

**Interpretation:** FlightCtl reported a status we don't recognize. Default to Pending to indicate we're waiting for a known state.

## Implementation

### Data Structures

Added to [pkg/flightctl/pods.go](../pkg/flightctl/pods.go):

```go
// FlightctlDeviceStatus represents the status section of a Device.
type FlightctlDeviceStatus struct {
    Applications []FlightctlApplicationStatus `json:"applications,omitempty"`
    Conditions   []FlightctlCondition          `json:"conditions,omitempty"`
}

// FlightctlApplicationStatus represents the runtime status of an application.
type FlightctlApplicationStatus struct {
    Name    string `json:"name"`              // Application name
    Status  string `json:"status"`            // running, pending, failed, etc.
    Summary string `json:"summary,omitempty"` // Human-readable message
}
```

### Status Mapping Function

```go
func (pm *PodManager) mapFlightctlStatusToPodStatus(appStatus *FlightctlApplicationStatus) *corev1.PodStatus {
    var phase corev1.PodPhase
    var conditions []corev1.PodCondition

    switch strings.ToLower(appStatus.Status) {
    case "running":
        phase = corev1.PodRunning
        conditions = []corev1.PodCondition{{
            Type:   corev1.PodReady,
            Status: corev1.ConditionTrue,
            Reason: "ApplicationRunning",
            Message: appStatus.Summary,
        }}
    // ... other cases ...
    }

    return &corev1.PodStatus{
        Phase:      phase,
        Conditions: conditions,
        Message:    appStatus.Summary,
    }
}
```

## Benefits

### 1. Accurate Status Reporting

Pods now reflect actual runtime state instead of just deployment intent:

```bash
# Before: Always shows Running if in spec
kubectl get pods
NAME         STATUS    READY
nginx-pod    Running   1/1    # Even if failed!

# After: Shows actual status from FlightCtl
kubectl get pods
NAME         STATUS    READY
nginx-pod    Running   1/1    # Actually running
app-failed   Failed    0/1    # Failed to start
app-pending  Pending   0/1    # Waiting to start
```

### 2. Failure Detection

Failed applications are now properly detected:

```bash
kubectl describe pod app-failed
...
Status:   Failed
Conditions:
  Type:   Ready
  Status: False
  Reason: ApplicationFailed
  Message: Container failed to start: image not found
```

### 3. Correct Lifecycle States

Applications transition through proper lifecycle states:

1. **CreatePod** → Pending (scheduled)
2. **FlightCtl starts app** → Pending (starting)
3. **App is running** → Running (ready)
4. **App crashes** → Failed
5. **App completes** → Succeeded

## Testing

### Manual Testing with FlightCtl

1. Deploy a pod:
```bash
kubectl apply -f test-pod.yaml
```

2. Check initial status (should be Pending):
```bash
kubectl get pod test-pod
# NAME       STATUS    READY
# test-pod   Pending   0/1
```

3. Wait for FlightCtl to start the app:
```bash
# Query the Device resource directly
curl -H "Authorization: Bearer $TOKEN" \
  https://flightctl-api/api/v1/devices/$DEVICE_ID

# Should show status.applications[].status = "running"
```

4. Verify pod is now Running:
```bash
kubectl get pod test-pod
# NAME       STATUS    READY
# test-pod   Running   1/1
```

### Status Reconciliation

The background reconciliation loop (every 15 seconds) automatically syncs status:

```go
// In provider.go
func (p *Provider) reconcilePodStatus() {
    for _, mapping := range mappings {
        // Queries GetPodStatus(), which now checks device.status.applications
        status, err := p.podManager.GetPodStatus(ctx, pod, mapping.DeviceID)
        // Updates cache
        cachedMapping.Status = status
    }
}
```

## Future Enhancements

1. **Container-level status**: Map individual containers in compose to container statuses
2. **Restart counts**: Track application restart count from FlightCtl
3. **Start time**: Report actual application start time from device
4. **Resource usage**: Include CPU/memory usage from device metrics
5. **Exit codes**: Report container exit codes for failed applications
6. **Event generation**: Generate Kubernetes events for status transitions

## Related Files

- Status mapping implementation: [pkg/flightctl/pods.go:154-242](../pkg/flightctl/pods.go)
- Status structures: [pkg/flightctl/pods.go:441-460](../pkg/flightctl/pods.go)
- Status reconciliation: [pkg/provider/provider.go:89-122](../pkg/provider/provider.go)
- Documentation: [POD_STATUS_MANAGEMENT.md](./POD_STATUS_MANAGEMENT.md)
