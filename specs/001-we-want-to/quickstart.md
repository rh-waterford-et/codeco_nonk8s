# Quickstart: Virtual Kubelet Flightctl Provider

**Feature**: Non-Native Kubernetes Workload Deployment
**Purpose**: Validate end-to-end integration between Kubernetes, Virtual Kubelet provider, and Flightctl
**Audience**: Developers, QA engineers, operators

---

## Prerequisites

Before running this quickstart, ensure you have:

1. **Kubernetes cluster** (kind recommended for local testing)
   ```bash
   kind create cluster --name vk-test
   ```

2. **Flightctl instance** accessible via API
   - API URL (e.g., `https://flightctl.example.com`)
   - Authentication token
   - At least one fleet with 2+ devices

3. **kubectl** configured for your cluster
   ```bash
   kubectl cluster-info
   ```

4. **Built provider binary or container image**
   ```bash
   # Option 1: Local binary
   go build -o bin/vk-flightctl-provider ./cmd/vk-flightctl-provider

   # Option 2: Container image
   docker build -t vk-flightctl-provider:latest .
   ```

---

## Setup (5 minutes)

### 1. Configure Flightctl Credentials

Create a Kubernetes secret with Flightctl API credentials:

```bash
kubectl create secret generic flightctl-credentials \
  --from-literal=api-url="https://flightctl.example.com" \
  --from-literal=auth-token="your-flightctl-token-here"
```

### 2. Deploy RBAC Configuration

Apply the provider's RBAC permissions:

```bash
kubectl apply -f config/rbac.yaml
```

Expected RBAC resources:
- ServiceAccount: `vk-flightctl-provider`
- ClusterRole: `vk-flightctl-provider`
- ClusterRoleBinding: `vk-flightctl-provider`

### 3. Deploy Virtual Kubelet Provider

```bash
kubectl apply -f config/deployment.yaml
```

This creates:
- Deployment: `vk-flightctl-provider` (1 replica)
- ConfigMap: `vk-flightctl-config` (provider settings)

### 4. Verify Provider is Running

```bash
# Check pod status
kubectl get pods -l app=vk-flightctl-provider

# Expected output:
# NAME                                     READY   STATUS    RESTARTS   AGE
# vk-flightctl-provider-7d8f9c5b6d-x7k2p   1/1     Running   0          30s

# Check provider logs
kubectl logs -l app=vk-flightctl-provider

# Expected log entries:
# {"level":"info","msg":"Provider starting","fleet":"edge-fleet-1"}
# {"level":"info","msg":"Connected to Flightctl API","url":"https://flightctl.example.com"}
# {"level":"info","msg":"Virtual node registered","node":"vk-flightctl-edge-fleet-1"}
```

### 5. Verify Virtual Node Appeared

```bash
kubectl get nodes

# Expected output includes virtual node:
# NAME                          STATUS   ROLES    AGE   VERSION
# kind-control-plane            Ready    master   10m   v1.28.0
# vk-flightctl-edge-fleet-1     Ready    agent    30s   v1.28.0-vk-flightctl
```

Check node capacity (aggregated from Flightctl devices):

```bash
kubectl describe node vk-flightctl-edge-fleet-1

# Expected capacity section:
# Capacity:
#   cpu:                8       # Total CPU from all devices in fleet
#   memory:             16Gi    # Total memory from all devices
#   pods:               110
# Allocatable:
#   cpu:                7500m   # Available CPU after system overhead
#   memory:             15Gi    # Available memory
```

---

## Test Scenario 1: Deploy Workload to Edge Device (2 minutes)

### Objective
Deploy a simple nginx pod to an edge device and verify it runs successfully.

### Steps

**1. Create a test pod targeting the virtual node:**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nginx-edge
  namespace: codeco
spec:
  nodeName: vk-flightctl-node
  containers:
  - name: nginx
    image: nginx:alpine
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
EOF
```

**2. Watch pod status:**

```bash
kubectl get pod nginx-edge --watch

# Expected progression:
# NAME         READY   STATUS    RESTARTS   AGE
# nginx-edge   0/1     Pending   0          0s
# nginx-edge   0/1     Pending   0          1s   # Provider selected device
# nginx-edge   0/1     Running   0          5s   # Workload deployed to device
# nginx-edge   1/1     Running   0          8s   # Container started
```

**3. Verify pod details:**

```bash
kubectl describe pod nginx-edge

# Expected events:
# Events:
#   Type    Reason     Age   From                    Message
#   ----    ------     ----  ----                    -------
#   Normal  Scheduled  30s   default-scheduler       Successfully assigned default/nginx-edge to vk-flightctl-edge-fleet-1
#   Normal  Pulling    28s   vk-flightctl-provider   Pulling image "nginx:alpine"
#   Normal  Pulled     25s   vk-flightctl-provider   Successfully pulled image
#   Normal  Created    25s   vk-flightctl-provider   Created container nginx
#   Normal  Started    24s   vk-flightctl-provider   Started container nginx

# Expected status:
# Status:             Running
# IP:                 <device-ip>
# Node:               vk-flightctl-edge-fleet-1/<device-id>
```

**4. Verify workload deployed to Flightctl:**

```bash
# Query Flightctl API (requires flightctl CLI or curl)
curl -H "Authorization: Bearer $FLIGHTCTL_TOKEN" \
  https://flightctl.example.com/api/v1/devices/<device-id>/workloads

# Expected response includes:
# {
#   "id": "default-nginx-edge-<uid>",
#   "name": "nginx-edge",
#   "namespace": "default",
#   "status": "Running",
#   "containers": [{"name": "nginx", "image": "nginx:alpine", "state": "Running"}]
# }
```

### Success Criteria
- ✅ Pod transitions to Running state within 10 seconds
- ✅ Pod events show successful deployment
- ✅ Workload appears in Flightctl device workload list
- ✅ Pod IP assigned from edge device network

---

## Test Scenario 2: Fleet-Level Targeting (3 minutes)

### Objective
Deploy multiple pod replicas across devices in a fleet using label selectors.

### Steps

**1. Label the virtual node with fleet identifier:**

```bash
# Node should already have fleet label from provider
kubectl get node vk-flightctl-edge-fleet-1 --show-labels

# Expected labels include:
# flightctl.io/fleet=edge-fleet-1
```

**2. Create a deployment targeting the fleet:**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-edge
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-edge
  template:
    metadata:
      labels:
        app: hello-edge
    spec:
      nodeSelector:
        flightctl.io/fleet: edge-fleet-1
      containers:
      - name: hello
        image: gcr.io/google-samples/hello-app:1.0
        resources:
          requests:
            cpu: "50m"
            memory: "64Mi"
EOF
```

**3. Verify pods distributed across devices:**

```bash
kubectl get pods -l app=hello-edge -o wide

# Expected output (pods on different devices):
# NAME                          READY   STATUS    NODE
# hello-edge-7d8f9c5b6d-abc12   1/1     Running   vk-flightctl-edge-fleet-1/device-1
# hello-edge-7d8f9c5b6d-def34   1/1     Running   vk-flightctl-edge-fleet-1/device-2
# hello-edge-7d8f9c5b6d-ghi56   1/1     Running   vk-flightctl-edge-fleet-1/device-1
```

**4. Verify resource allocation:**

```bash
kubectl describe node vk-flightctl-edge-fleet-1

# Expected Allocated resources section shows pod requests:
# Allocated resources:
#   Resource  Requests    Limits
#   --------  --------    ------
#   cpu       250m (3%)   0 (0%)      # 50m * 3 pods + 100m from nginx-edge
#   memory    320Mi (2%)  0 (0%)      # 64Mi * 3 pods + 128Mi from nginx-edge
```

### Success Criteria
- ✅ All 3 replica pods reach Running state
- ✅ Pods distributed across multiple devices (if fleet has 2+ devices)
- ✅ Node allocatable resources updated correctly
- ✅ Workloads visible in Flightctl for each device

---

## Test Scenario 3: Resource Validation (2 minutes)

### Objective
Verify that pods exceeding device capacity are rejected with clear error messages.

### Steps

**1. Check current allocatable resources:**

```bash
kubectl describe node vk-flightctl-edge-fleet-1 | grep Allocatable -A 3

# Note the available CPU/memory
```

**2. Create a pod requesting excessive resources:**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: resource-hog
  namespace: default
spec:
  nodeName: vk-flightctl-edge-fleet-1
  containers:
  - name: hog
    image: nginx:alpine
    resources:
      requests:
        cpu: "100"       # Request 100 CPU cores (exceeds capacity)
        memory: "1000Gi" # Request 1000 GB memory
EOF
```

**3. Verify pod is rejected:**

```bash
kubectl get pod resource-hog

# Expected status:
# NAME           READY   STATUS   RESTARTS   AGE
# resource-hog   0/1     Failed   0          5s

kubectl describe pod resource-hog

# Expected event:
# Events:
#   Type     Reason            Age   From                    Message
#   ----     ------            ----  ----                    -------
#   Warning  FailedScheduling  10s   vk-flightctl-provider   Insufficient resources: requested cpu=100, memory=1000Gi; available cpu=7500m, memory=15Gi
```

**4. Clean up failed pod:**

```bash
kubectl delete pod resource-hog
```

### Success Criteria
- ✅ Pod fails with "Insufficient resources" reason
- ✅ Error message shows requested vs. available resources
- ✅ Provider doesn't attempt to deploy to Flightctl
- ✅ Pod deletion succeeds

---

## Test Scenario 4: Workload Update (Simple Replace) (3 minutes)

### Objective
Update a running pod and verify simple replace strategy (stop old, start new).

### Steps

**1. Deploy initial workload:**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: update-test
  namespace: default
  labels:
    version: v1
spec:
  nodeName: vk-flightctl-edge-fleet-1
  containers:
  - name: app
    image: gcr.io/google-samples/hello-app:1.0
    env:
    - name: VERSION
      value: "1.0"
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
EOF

# Wait for pod to be Running
kubectl wait --for=condition=Ready pod/update-test --timeout=30s
```

**2. Update pod (change image and env var):**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: update-test
  namespace: default
  labels:
    version: v2
spec:
  nodeName: vk-flightctl-edge-fleet-1
  containers:
  - name: app
    image: gcr.io/google-samples/hello-app:2.0
    env:
    - name: VERSION
      value: "2.0"
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
EOF
```

**3. Watch pod status during update:**

```bash
kubectl get pod update-test --watch

# Expected progression (simple replace):
# NAME          READY   STATUS    RESTARTS   AGE
# update-test   1/1     Running   0          30s
# update-test   1/1     Terminating   0      35s   # Old workload stopping
# update-test   0/1     Pending       0      36s   # New workload deploying
# update-test   0/1     Running       0      40s   # New workload starting
# update-test   1/1     Running       0      42s   # New workload ready
```

**4. Verify updated version:**

```bash
kubectl describe pod update-test | grep Image:

# Expected output:
# Image:          gcr.io/google-samples/hello-app:2.0
```

### Success Criteria
- ✅ Old workload terminated before new one starts (no overlap)
- ✅ Pod transitions to Running with updated image
- ✅ Update completes within 15 seconds
- ✅ No downtime exceeds simple replace duration

---

## Test Scenario 5: Device Disconnection and Reconnection (5 minutes)

### Objective
Verify timeout-based reconnection behavior when a device becomes unavailable.

### Steps

**1. Deploy a workload to a specific device:**

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: resilience-test
  namespace: default
spec:
  nodeName: vk-flightctl-edge-fleet-1
  nodeSelector:
    flightctl.io/device-id: device-1  # Target specific device
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: "50m"
        memory: "64Mi"
EOF

kubectl wait --for=condition=Ready pod/resilience-test --timeout=30s
```

**2. Simulate device disconnection:**

```bash
# This step requires Flightctl CLI or API access to mark device offline
# For testing: Use mock Flightctl server to simulate disconnection

# If using real Flightctl, disconnect device network or stop device agent
```

**3. Observe pod status changes:**

```bash
kubectl get pod resilience-test --watch

# Expected progression:
# NAME              READY   STATUS    RESTARTS   AGE
# resilience-test   1/1     Running   0          1m
# resilience-test   1/1     Unknown   0          1m30s  # Device disconnected
```

**4. Wait for reconnection timeout (default 5 minutes):**

```bash
# Monitor provider logs
kubectl logs -l app=vk-flightctl-provider --tail=50 -f

# Expected log entries:
# {"level":"warn","msg":"Device disconnected","device":"device-1","pod":"default/resilience-test"}
# {"level":"info","msg":"Starting reconnection timeout","device":"device-1","timeout":"5m"}
# ... (wait 5 minutes) ...
# {"level":"warn","msg":"Reconnection timeout exceeded","device":"device-1"}
# {"level":"info","msg":"Rescheduling workload","pod":"default/resilience-test","reason":"DeviceTimeout"}
```

**5. Verify pod is rescheduled:**

```bash
kubectl get pod resilience-test -o wide

# Expected: Pod deleted and Kubernetes reschedules to another node
# (If no other suitable nodes, pod remains Pending)
```

**6. Test early reconnection (before timeout):**

```bash
# Redeploy pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: reconnect-test
  namespace: default
spec:
  nodeName: vk-flightctl-edge-fleet-1
  nodeSelector:
    flightctl.io/device-id: device-1
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: "50m"
        memory: "64Mi"
EOF

# Simulate disconnection (as before)
# Reconnect device BEFORE timeout expires (e.g., after 2 minutes)

# Expected behavior:
kubectl get pod reconnect-test

# Pod status should return to Running (not rescheduled)
# Logs should show:
# {"level":"info","msg":"Device reconnected","device":"device-1","downtime":"2m15s"}
# {"level":"info","msg":"Cancelled reconnection timeout","device":"device-1"}
# {"level":"info","msg":"Workload status synchronized","pod":"default/reconnect-test"}
```

### Success Criteria
- ✅ Pod status changes to Unknown when device disconnects
- ✅ If timeout expires: Pod deleted and rescheduled
- ✅ If device reconnects before timeout: Pod status restored, no rescheduling
- ✅ Timeout duration is configurable (verify via ConfigMap)
- ✅ Provider logs all disconnection/reconnection events

---

## Test Scenario 6: Pod Deletion and Cleanup (1 minute)

### Objective
Verify that deleting a pod removes the workload from the edge device.

### Steps

**1. List current pods:**

```bash
kubectl get pods -o wide
```

**2. Delete a pod:**

```bash
kubectl delete pod nginx-edge
```

**3. Verify pod deleted from Kubernetes:**

```bash
kubectl get pod nginx-edge

# Expected output:
# Error from server (NotFound): pods "nginx-edge" not found
```

**4. Verify workload removed from Flightctl:**

```bash
# Query Flightctl API
curl -H "Authorization: Bearer $FLIGHTCTL_TOKEN" \
  https://flightctl.example.com/api/v1/devices/<device-id>/workloads

# Workload "default-nginx-edge-<uid>" should NOT appear in list
```

**5. Verify resources freed:**

```bash
kubectl describe node vk-flightctl-edge-fleet-1

# Allocated resources should decrease by nginx-edge's requests (100m CPU, 128Mi memory)
```

### Success Criteria
- ✅ Pod deleted from Kubernetes within 5 seconds
- ✅ Workload removed from Flightctl device within 10 seconds
- ✅ Node allocatable resources updated
- ✅ Provider logs deletion event

---

## Cleanup

Remove all test resources:

```bash
# Delete test pods and deployments
kubectl delete deployment hello-edge
kubectl delete pod update-test resilience-test reconnect-test --ignore-not-found

# Delete provider
kubectl delete -f config/deployment.yaml
kubectl delete -f config/rbac.yaml

# Delete credentials
kubectl delete secret flightctl-credentials

# Delete kind cluster (if used)
kind delete cluster --name vk-test
```

---

## Troubleshooting

### Provider pod not starting

**Check logs:**
```bash
kubectl logs -l app=vk-flightctl-provider
```

**Common issues:**
- Invalid Flightctl credentials: Check secret values
- Network connectivity: Verify Flightctl API is reachable from cluster
- RBAC permissions: Verify ServiceAccount has required permissions

### Virtual node not appearing

**Check provider logs:**
```bash
kubectl logs -l app=vk-flightctl-provider | grep "node registered"
```

**Common issues:**
- Fleet has no devices: Check Flightctl fleet has at least one device
- Provider crashed during startup: Check logs for errors

### Pod stuck in Pending

**Check events:**
```bash
kubectl describe pod <pod-name>
```

**Common issues:**
- Insufficient resources: Check node allocatable vs. pod requests
- No matching devices: Check nodeSelector/labels match Flightctl devices
- Device offline: Check device connectivity in Flightctl

### Pod status not updating

**Check provider reconciliation:**
```bash
kubectl logs -l app=vk-flightctl-provider | grep reconcile
```

**Common issues:**
- Flightctl API slow/unreachable: Check network connectivity
- Status poll interval too long: Adjust via ConfigMap
- Provider crashed: Check for errors in logs

---

## Success Criteria Summary

This quickstart is successful if all test scenarios pass:

1. ✅ **Deployment**: nginx pod deploys and runs on edge device
2. ✅ **Fleet Targeting**: 3 replicas distribute across fleet devices
3. ✅ **Resource Validation**: Excessive requests rejected with clear errors
4. ✅ **Update**: Pod updates using simple replace strategy
5. ✅ **Disconnection**: Timeout-based reconnection and rescheduling works
6. ✅ **Deletion**: Pod deletion removes workload from device

**Total time**: ~20 minutes

**Next steps**:
- Run integration test suite: `go test ./tests/integration/...`
- Deploy to production Kubernetes cluster
- Configure monitoring (Prometheus metrics at `/metrics`)
- Set up alerting for device disconnections and workload failures
