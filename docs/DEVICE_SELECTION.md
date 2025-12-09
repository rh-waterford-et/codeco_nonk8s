# Device Selection with Annotations

This document explains how to control which FlightCtl device or fleet a pod is deployed to using Kubernetes annotations.

## Overview

The Virtual Kubelet FlightCtl provider supports selecting target devices through pod annotations. This allows fine-grained control over pod placement on edge devices.

## Supported Annotations

### Device ID Annotation

Deploy a pod to a specific FlightCtl device:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  namespace: default
  annotations:
    flightctl.io/device-id: "d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0"
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

**Key:** `flightctl.io/device-id`
**Value:** FlightCtl device identifier (string)
**Use Case:** Direct device targeting, testing, specific hardware requirements

### Fleet ID Annotation (Planned)

Deploy a pod to any device in a FlightCtl fleet:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  namespace: default
  annotations:
    flightctl.io/fleet-id: "production-fleet-east"
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

**Key:** `flightctl.io/fleet-id`
**Value:** FlightCtl fleet identifier (string)
**Status:** Not yet implemented (returns error)
**Use Case:** Load balancing across device groups, geographic distribution

## Selection Priority

The provider checks annotations in this order:

1. **`flightctl.io/device-id`** - If present, deploy to this specific device
2. **`flightctl.io/fleet-id`** - If present (and no device-id), select a device from this fleet
3. **Default device** - If no annotations, use default device: `d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0`

## Examples

### Example 1: Deploy to Specific Device

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: edge-camera-app
  namespace: manufacturing
  annotations:
    flightctl.io/device-id: "device-camera-01"
spec:
  containers:
  - name: camera-processor
    image: camera-app:v1.2
    ports:
    - containerPort: 8080
```

**Result:** Pod deploys to device `device-camera-01`

### Example 2: No Annotations (Default)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: simple-app
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

**Result:** Pod deploys to default device `d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0`

### Example 3: Fleet-based (Not Yet Supported)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: distributed-app
  namespace: default
  annotations:
    flightctl.io/fleet-id: "retail-stores-west"
spec:
  containers:
  - name: pos-system
    image: retail-pos:v2.1
```

**Result:** Currently returns error: "fleet-based device selection not yet implemented"

## Implementation Details

### Device Selection Logic

Location: [pkg/provider/provider.go:140-170](../pkg/provider/provider.go#L140-L170)

```go
func (p *Provider) selectDeviceForPod(pod *corev1.Pod) (string, error) {
    const (
        deviceIDAnnotation = "flightctl.io/device-id"
        fleetIDAnnotation  = "flightctl.io/fleet-id"
        defaultDeviceID    = "d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0"
    )

    // Check for direct device ID annotation
    if deviceID, ok := pod.Annotations[deviceIDAnnotation]; ok && deviceID != "" {
        return deviceID, nil
    }

    // Check for fleet ID annotation
    if fleetID, ok := pod.Annotations[fleetIDAnnotation]; ok && fleetID != "" {
        // TODO: Implement fleet selection
        return "", fmt.Errorf("fleet-based device selection not yet implemented")
    }

    // No annotations - use default device
    return defaultDeviceID, nil
}
```

### Where It's Called

The device selection happens in [CreatePod()](../pkg/provider/provider.go#L175):

```go
func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
    // Select device from pod annotations or use default
    deviceID, err := p.selectDeviceForPod(pod)
    if err != nil {
        return fmt.Errorf("selecting device for pod: %w", err)
    }

    // Deploy to selected device
    if err := p.podManager.DeployPod(ctx, pod, deviceID); err != nil {
        return fmt.Errorf("deploying pod to device %s: %w", deviceID, err)
    }
}
```

## Error Handling

### Invalid Device ID

If the device ID doesn't exist, the deployment will fail during the FlightCtl API call:

```bash
kubectl get pods
NAME           STATUS    READY
my-app         Pending   0/1

kubectl describe pod my-app
Events:
  Warning  FailedCreate  1s  virtual-kubelet  Error deploying pod to device invalid-device: GET device failed with status 404
```

### Fleet Not Implemented

```bash
kubectl describe pod distributed-app
Events:
  Warning  FailedCreate  1s  virtual-kubelet  Error selecting device for pod: fleet-based device selection not yet implemented (fleet: retail-stores-west)
```

## Best Practices

### 1. Use Device ID for Specific Hardware

When your application requires specific hardware (GPU, sensors, etc.):

```yaml
annotations:
  flightctl.io/device-id: "device-with-gpu-01"
```

### 2. Document Device IDs

Keep a registry of device IDs and their capabilities:

```yaml
# devices.yaml
devices:
  device-with-gpu-01:
    capabilities: [gpu, high-memory]
    location: warehouse-a
  device-camera-01:
    capabilities: [camera, edge-ai]
    location: production-line-3
```

### 3. Use Labels for Grouping

Combine FlightCtl annotations with Kubernetes labels:

```yaml
metadata:
  labels:
    app: camera-processor
    location: warehouse-a
    hardware: gpu-enabled
  annotations:
    flightctl.io/device-id: "device-with-gpu-01"
```

### 4. Validate Device IDs

Before deploying, verify the device exists:

```bash
# Query FlightCtl API
curl -H "Authorization: Bearer $TOKEN" \
  https://flightctl-api/api/v1/devices/device-with-gpu-01
```

## Configuration

### Changing the Default Device

The default device ID is hardcoded in the provider. To change it, modify:

```go
// pkg/provider/provider.go
const defaultDeviceID = "your-device-id-here"
```

**Future Enhancement:** Make this configurable via environment variable or config file.

## Future Enhancements

### Fleet-based Selection

When implemented, fleet selection will:

1. Query FlightCtl API for all devices in the fleet
2. Select a device based on:
   - Available capacity (CPU, memory, pods)
   - Current workload
   - Device health status
   - Load balancing algorithm (round-robin, least-loaded, etc.)
3. Return the selected device ID

### Load Balancing Strategies

Possible strategies for fleet selection:

- **Round-robin**: Cycle through devices in fleet
- **Least-loaded**: Choose device with most available resources
- **Affinity-based**: Prefer devices in same location/zone
- **Random**: Randomly select from healthy devices

### Device Affinity/Anti-affinity

Similar to Kubernetes node affinity:

```yaml
annotations:
  flightctl.io/device-selector: |
    matchLabels:
      hardware: gpu
      location: warehouse-a
    matchExpressions:
    - key: memory
      operator: GreaterThan
      values: ["16Gi"]
```

## Troubleshooting

### Pod Stuck in Pending

**Symptom:** Pod status remains Pending

**Check:**
1. Device ID is correct: `curl https://flightctl-api/api/v1/devices/{device-id}`
2. Provider logs: `kubectl logs -n kube-system <virtual-kubelet-pod>`
3. FlightCtl device is online: Check device status in FlightCtl UI

### Device Not Accepting Pods

**Symptom:** Deployment succeeds but pod doesn't start on device

**Check:**
1. FlightCtl agent running on device
2. Device has capacity for pod
3. Network connectivity between device and FlightCtl API
4. Check device logs on the edge device

## Related Documentation

- [Pod Status Management](./POD_STATUS_MANAGEMENT.md) - How pod status is tracked
- [FlightCtl Status Mapping](./FLIGHTCTL_STATUS_MAPPING.md) - Status translation
- [Pod to Compose Conversion](./POD_TO_COMPOSE_CONVERSION.md) - How pods are converted

## API Reference

### Annotation Keys

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `flightctl.io/device-id` | string | No | Target device identifier |
| `flightctl.io/fleet-id` | string | No | Target fleet identifier (not implemented) |

### Default Values

| Setting | Value | Configurable |
|---------|-------|--------------|
| Default Device ID | `d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0` | No (hardcoded) |
| Selection Priority | device-id → fleet-id → default | No |
