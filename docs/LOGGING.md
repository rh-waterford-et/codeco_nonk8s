# Logging

This document explains the logging system used in the Virtual Kubelet FlightCtl provider.

## Overview

The provider uses a simple, leveled logging system defined in [pkg/logger/logger.go](../pkg/logger/logger.go). This provides consistent, structured logging across all components.

## Log Levels

The logger supports four log levels, from most to least verbose:

| Level | Use Case | Example |
|-------|----------|---------|
| **DEBUG** | Verbose diagnostic information, frequent operations | API payloads, device queries, getter methods |
| **INFO** | Normal operational messages | Pod creation, device updates, successful operations |
| **WARN** | Warning conditions that don't prevent operation | Deprecated features, fallback behavior |
| **ERROR** | Error conditions | Failed API calls, invalid configurations |

Additionally, there's a **FATAL** level that logs and immediately exits the program.

## Configuration

### Setting Log Level

The log level can be configured via the `LOG_LEVEL` environment variable:

```bash
# In Kubernetes deployment
env:
- name: LOG_LEVEL
  value: "debug"

# Or when running directly
export LOG_LEVEL=debug
./vk-flightctl-provider
```

Supported values: `debug`, `info`, `warn`, `error` (case-insensitive)

Default: `info`

### Log Format

All log messages follow this format:

```
2025/01/09 14:23:45 [LEVEL] message
```

Example output:
```
2025/01/09 14:23:45 [INFO] Provider Create Pod nginx-pod
2025/01/09 14:23:45 [DEBUG] Retrieve Device info from flightctl
2025/01/09 14:23:46 [INFO] Successfully updated device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0
2025/01/09 14:23:46 [INFO] Pod default/nginx-pod created with initial Pending status
```

## Usage in Code

### Importing

```go
import "github.com/raycarroll/vk-flightctl-provider/pkg/logger"
```

### Basic Logging

```go
// Debug level (verbose)
logger.Debug("Retrieve Device info from flightctl")
logger.Debug("Device payload: %s", jsonPayload)

// Info level (normal operations)
logger.Info("Provider Create Pod %s", pod.Name)
logger.Info("Deploying pod %s to device %s", podKey, deviceID)

// Warning level
logger.Warn("Fleet selection not yet implemented, using default device")

// Error level
logger.Error("Failed to get status for pod %s/%s: %v", namespace, name, err)

// Fatal (logs and exits)
logger.Fatal("Required configuration missing: %s", configKey)
```

### Format Strings

The logger uses `fmt.Printf`-style format strings:

```go
// String (%s)
logger.Info("Pod name: %s", podName)

// Integer (%d)
logger.Info("Updated device with %d applications", len(apps))

// Error (%v or %w)
logger.Error("Deployment failed: %v", err)

// Multiple values
logger.Info("Deploying pod %s to device %s", podKey, deviceID)
```

**Important:** Do NOT add `\n` at the end of format strings - the logger adds newlines automatically.

```go
// ❌ Wrong
logger.Info("Pod created\n")

// ✅ Correct
logger.Info("Pod created")
```

### Prefixed Logger

For components that need a consistent prefix:

```go
// Create a prefixed logger
podLogger := logger.WithPrefix("[PodManager] ")

// All messages will include the prefix
podLogger.Info("Deploying pod")  // Outputs: [INFO] [PodManager] Deploying pod
podLogger.Error("Deploy failed") // Outputs: [ERROR] [PodManager] Deploy failed
```

## Log Level Guidelines

### When to Use DEBUG

Use DEBUG for:
- Verbose diagnostic information
- Frequently called operations (getters, status checks)
- Detailed payloads and API responses
- Information useful for troubleshooting but too noisy for production

Examples:
```go
logger.Debug("Provider Get Pod %s", name)
logger.Debug("Updating device %s with payload:\n%s", deviceID, payload)
logger.Debug("PodToCompose:\n%s", composeYAML)
```

### When to Use INFO

Use INFO for:
- Normal operational events
- Pod lifecycle operations (create, update, delete)
- Device updates
- Successful operations
- System startup/shutdown

Examples:
```go
logger.Info("Provider Create Pod %s", pod.Name)
logger.Info("Successfully updated device %s", deviceID)
logger.Info("Pod %s created with initial Pending status", podKey)
logger.Info("Status reconciliation loop stopped")
```

### When to Use WARN

Use WARN for:
- Non-critical issues
- Fallback to default behavior
- Deprecated features
- Unusual but handleable conditions

Examples:
```go
logger.Warn("Unknown log level %s, using info", level)
logger.Warn("Fleet selection not implemented, using default device")
```

### When to Use ERROR

Use ERROR for:
- API call failures
- Invalid data
- Resource not found
- Operations that failed but don't crash the system

Examples:
```go
logger.Error("Failed to get status for pod %s/%s: %v", namespace, name, err)
logger.Error("GET device failed with status %d: %s", statusCode, body)
logger.Error("decoding device: %s", err)
```

## Log Output Examples

### INFO Level (Default)

```
2025/01/09 14:23:45 [INFO] Provider Create Pod nginx-pod
2025/01/09 14:23:45 [INFO] Deploying pod default/nginx-pod to device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0
2025/01/09 14:23:46 [INFO] Updated device with 1 applications
2025/01/09 14:23:46 [INFO] Successfully updated device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0
2025/01/09 14:23:46 [INFO] Pod default/nginx-pod created with initial Pending status
```

### DEBUG Level

```
2025/01/09 14:23:45 [DEBUG] Provider Get Pod nginx-pod
2025/01/09 14:23:45 [DEBUG] Retrieve Device info from flightctl
2025/01/09 14:23:45 [DEBUG] Converting Pod to FlightCTL App Spec
2025/01/09 14:23:45 [DEBUG] Creating Inline Content Section
2025/01/09 14:23:45 [DEBUG] PodToCompose:
 version: '3.8'
 services:
  nginx:
    image: nginx:1.21
    ...
2025/01/09 14:23:46 [DEBUG] Updating device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0 with payload:
{"apiVersion":"flightctl.io/v1alpha1",...}
2025/01/09 14:23:46 [INFO] Successfully updated device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0
```

### ERROR Level

```
2025/01/09 14:23:45 [INFO] Provider Create Pod nginx-pod
2025/01/09 14:23:45 [ERROR] getting device d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0: GET device failed with status 404: {"error":"device not found"}
```

## Deployment Examples

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: virtual-kubelet-flightctl
spec:
  template:
    spec:
      containers:
      - name: virtual-kubelet
        image: vk-flightctl-provider:latest
        env:
        # Production - INFO level
        - name: LOG_LEVEL
          value: "info"

        # Or development - DEBUG level
        # - name: LOG_LEVEL
        #   value: "debug"
```

### Docker Run

```bash
# Production
docker run -e LOG_LEVEL=info vk-flightctl-provider:latest

# Development
docker run -e LOG_LEVEL=debug vk-flightctl-provider:latest
```

## Viewing Logs

### Kubernetes

```bash
# Follow logs
kubectl logs -f -n kube-system deployment/virtual-kubelet-flightctl

# Get recent logs
kubectl logs --tail=100 -n kube-system deployment/virtual-kubelet-flightctl

# Filter by log level
kubectl logs -n kube-system deployment/virtual-kubelet-flightctl | grep "\[ERROR\]"
kubectl logs -n kube-system deployment/virtual-kubelet-flightctl | grep "\[WARN\]"
```

### Docker

```bash
# Follow logs
docker logs -f <container-id>

# Filter by log level
docker logs <container-id> | grep "\[ERROR\]"
```

## Programmatic Access

### Getting Current Log Level

```go
currentLevel := logger.GetLevel()
fmt.Printf("Current log level: %s\n", currentLevel)
```

### Setting Log Level Programmatically

```go
// Set from string
logger.SetLevelFromString("debug")

// Set from LogLevel constant
logger.SetLevel(logger.DebugLevel)
```

## Migration from Old Logging

The codebase has been migrated from ad-hoc `println` and `fmt.Printf` statements to structured logging:

### Before

```go
println("Provider Create Pod ", pod.Name)
fmt.Printf("Deploying pod %s to device %s\n", podKey, deviceID)
fmt.Println("Status reconciliation loop stopped")
```

### After

```go
logger.Info("Provider Create Pod %s", pod.Name)
logger.Info("Deploying pod %s to device %s", podKey, deviceID)
logger.Info("Status reconciliation loop stopped")
```

### Benefits

1. **Consistent format**: All logs follow the same structure
2. **Log levels**: Can filter by severity
3. **Production control**: Set `LOG_LEVEL=error` to reduce noise
4. **Debug mode**: Set `LOG_LEVEL=debug` for troubleshooting
5. **No trailing newlines**: Logger handles formatting automatically

## Best Practices

### DO

✅ Use appropriate log levels based on the guidelines above
✅ Include context in log messages (pod name, device ID, etc.)
✅ Use format strings for values: `logger.Info("Pod %s created", name)`
✅ Log errors with context: `logger.Error("Failed to deploy: %v", err)`
✅ Use DEBUG for verbose diagnostic output

### DON'T

❌ Add `\n` to the end of format strings
❌ Use `println` or `fmt.Printf` directly
❌ Log sensitive data (passwords, tokens, etc.)
❌ Use INFO level for very frequent operations
❌ Over-log in production (use DEBUG for verbose output)

## Troubleshooting

### Logs Not Appearing

**Check log level:**
```bash
# Ensure LOG_LEVEL is set appropriately
kubectl set env deployment/virtual-kubelet-flightctl LOG_LEVEL=debug
```

### Too Much Log Output

**Reduce verbosity:**
```bash
# Set to INFO or ERROR level
kubectl set env deployment/virtual-kubelet-flightctl LOG_LEVEL=info
```

### Need More Detail

**Enable debug logging:**
```bash
# Set to DEBUG level
kubectl set env deployment/virtual-kubelet-flightctl LOG_LEVEL=debug
```

## Related Files

- Logger implementation: [pkg/logger/logger.go](../pkg/logger/logger.go)
- Provider logging: [pkg/provider/provider.go](../pkg/provider/provider.go)
- FlightCtl client logging: [pkg/flightctl/pods.go](../pkg/flightctl/pods.go)
