# Kubernetes Pod to Docker Compose Conversion

This document explains how the `convertPodToDockerCompose()` function converts Kubernetes Pod specifications to Docker Compose format for deployment via FlightCtl.

## Overview

The conversion function transforms Kubernetes Pod YAML into Docker Compose YAML that can be deployed to edge devices using FlightCtl's `applications` field with `appType: "compose"`.

## Feature Mapping

| Kubernetes Pod Feature | Docker Compose Equivalent | Notes |
|------------------------|---------------------------|-------|
| `spec.containers[].name` | Service name | Sanitized to lowercase with hyphens |
| `spec.containers[].image` | `image` | Direct mapping |
| `spec.containers[].command` | `entrypoint` | Array format |
| `spec.containers[].args` | `command` | Array format |
| `spec.containers[].env` | `environment` | Direct values only (secrets/configmaps as comments) |
| `spec.containers[].ports` | `ports` | Container port mapped to same host port |
| `spec.containers[].volumeMounts` | `volumes` (service level) | Includes read-only flag |
| `spec.containers[].resources.limits` | `deploy.resources.limits` | CPU and memory |
| `spec.containers[].resources.requests` | `deploy.resources.reservations` | CPU and memory |
| `spec.restartPolicy` | `restart` | Always→unless-stopped, Never→no, OnFailure→on-failure |
| `spec.volumes` | `volumes` (top level) | EmptyDir→named volume, HostPath→bind mount |

## Example 1: Simple NGINX Pod

### Input (Kubernetes Pod)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-pod
  namespace: default
spec:
  restartPolicy: Always
  containers:
  - name: nginx
    image: nginx:1.21
    ports:
    - containerPort: 80
    - containerPort: 443
    env:
    - name: NGINX_HOST
      value: example.com
    - name: NGINX_PORT
      value: "80"
    resources:
      limits:
        cpu: 500m
        memory: 512Mi
      requests:
        cpu: 250m
        memory: 256Mi
    volumeMounts:
    - name: html-volume
      mountPath: /usr/share/nginx/html
    - name: config-volume
      mountPath: /etc/nginx/conf.d
      readOnly: true
  volumes:
  - name: html-volume
    emptyDir: {}
  - name: config-volume
    configMap:
      name: nginx-config
```

### Output (Docker Compose)

```yaml
version: '3.8'
services:
  nginx:
    image: nginx:1.21
    environment:
      - NGINX_HOST=example.com
      - NGINX_PORT=80
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - html-volume:/usr/share/nginx/html
      - config-volume:/etc/nginx/conf.d:ro
    deploy:
      resources:
        limits:
          cpus: '500m'
          memory: 512Mi
        reservations:
          cpus: '250m'
          memory: 256Mi
    restart: unless-stopped

volumes:
  html-volume:
  # config-volume: from configmap nginx-config
```

## Example 2: Multi-Container Pod (Sidecar Pattern)

### Input (Kubernetes Pod)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-sidecar
spec:
  containers:
  - name: app
    image: myapp:v1.0
    ports:
    - containerPort: 8080
  - name: logging-sidecar
    image: fluent/fluentd:v1.14
    args:
    - -c
    - /fluentd/etc/fluent.conf
```

### Output (Docker Compose)

```yaml
version: '3.8'
services:
  app:
    image: myapp:v1.0
    ports:
      - "8080:8080"
    restart: unless-stopped

  logging-sidecar:
    image: fluent/fluentd:v1.14
    command:
      - -c
      - /fluentd/etc/fluent.conf
    restart: unless-stopped
```

## How It's Used in FlightCtl

When a Kubernetes Pod is created and assigned to the Virtual Kubelet provider, the following happens:

1. **Pod received** by Virtual Kubelet
2. **Device selected** (currently hardcoded to `oo934b6etggpv7fkaqi248k1jmf8iu3aj74di15nncafsotjh6rg`)
3. **Conversion to Docker Compose** using `convertPodToDockerCompose()`
4. **Application created** in FlightCtl Device resource:

```json
{
  "apiVersion": "flightctl.io/v1alpha1",
  "kind": "Device",
  "metadata": {
    "name": "oo934b6etggpv7fkaqi248k1jmf8iu3aj74di15nncafsotjh6rg"
  },
  "spec": {
    "applications": [
      {
        "name": "default-nginx-pod",
        "image": "nginx:1.21",
        "appType": "compose",
        "inline": "version: '3.8'\nservices:\n  nginx:\n    image: nginx:1.21\n..."
      }
    ]
  }
}
```

5. **Device applies** the compose file via FlightCtl agent
6. **Containers run** on edge device using Docker Compose

## Limitations

### Not Supported (Yet)

- **Init containers** - Would need separate service with depends_on
- **Readiness/Liveness probes** - No direct Docker Compose equivalent
- **SecurityContext** - Limited support in Docker Compose
- **Pod affinity/anti-affinity** - Not applicable for single device
- **ServiceAccounts** - Kubernetes-specific concept
- **Complex volume types** - PVC, CSI, etc. not supported
- **Environment from ConfigMaps/Secrets** - Marked as comments only

### Workarounds

1. **Secrets/ConfigMaps**: Pre-create them on the device or use environment variables directly
2. **Persistent volumes**: Use named volumes or host paths
3. **Health checks**: Add manual healthcheck sections to compose (future enhancement)

## Future Enhancements

- [ ] Support for Docker Compose healthchecks (from K8s probes)
- [ ] Network policy translation
- [ ] Support for init containers as dependencies
- [ ] Better handling of secrets (integration with FlightCtl secret management)
- [ ] Pod DNS configuration
- [ ] Host networking mode
- [ ] Privileged containers
- [ ] Device plugins / resource requests beyond CPU/memory

## Testing

Run the tests to see conversion examples:

```bash
cd /home/raycarroll/Documents/Code/codeco-nonk8-1
go test -v ./pkg/flightctl -run TestConvertPodToDockerCompose
```

## Code Location

- Implementation: [`pkg/flightctl/pods.go:202-352`](../pkg/flightctl/pods.go)
- Tests: [`pkg/flightctl/pods_test.go`](../pkg/flightctl/pods_test.go)
- Usage: [`pkg/flightctl/pods.go:219-221`](../pkg/flightctl/pods.go) in `podToFlightctlApplication()`
