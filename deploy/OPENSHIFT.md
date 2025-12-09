# OpenShift Deployment Guide

This guide covers deploying the VK-Flightctl Provider on OpenShift Container Platform.

## OpenShift-Specific Changes

The deployment has been configured to work with OpenShift's security model:

### Security Context

**Container Security Context:**
- ✅ `runAsNonRoot: true` - Required by OpenShift restricted SCC
- ✅ `allowPrivilegeEscalation: false` - Security best practice
- ✅ `capabilities.drop: [ALL]` - Drops all Linux capabilities
- ✅ `readOnlyRootFilesystem: true` - Prevents container writes
- ❌ `runAsUser` - **Removed** - OpenShift assigns random UID from namespace range

**Pod Security Context:**
- ✅ `runAsNonRoot: true` - Enforces non-root execution
- ✅ `seccompProfile.type: RuntimeDefault` - Uses default seccomp profile
- ❌ `fsGroup` - Not specified, OpenShift handles this

### Dockerfile

The Dockerfile is OpenShift-compatible:
```dockerfile
# Binary owned by UID 65532, group 0 (root group)
COPY --from=builder --chown=65532:0 --chmod=0750 \
    /workspace/vk-flightctl-provider /usr/local/bin/vk-flightctl-provider

# Default user (overridden by OpenShift)
USER 65532:0
```

**Why group 0?**
- OpenShift runs containers with a random UID from the namespace's allocated range
- All containers run with GID 0 (root group) for file access
- Files must be readable/executable by group 0

## Deployment Steps

### 1. Create Namespace/Project

```bash
oc new-project codeco
```

### 2. Create OAuth Secret

```bash
oc create secret generic vk-flightctl-oauth \
  --from-literal=client-id=YOUR_CLIENT_ID \
  --from-literal=client-secret=YOUR_CLIENT_SECRET \
  -n codeco
```

Or use the Makefile:
```bash
make create-secret CLIENT_ID=xxx CLIENT_SECRET=yyy NAMESPACE=codeco
```

### 3. Deploy Resources

```bash
# Apply all manifests
oc apply -f deploy/

# Or use kustomize
oc apply -k deploy/
```

### 4. Verify Deployment

```bash
# Check pod status
oc get pods -n codeco -l app=vk-flightctl-provider

# View logs
oc logs -n codeco -l app=vk-flightctl-provider -f

# Check if virtual node is registered
oc get nodes
```

## Security Context Constraints (SCC)

The deployment works with OpenShift's default **restricted-v2** SCC (OpenShift 4.11+) or **restricted** SCC (older versions).

### Verify SCC Assignment

```bash
# Check which SCC is being used
oc get pod -n codeco -l app=vk-flightctl-provider -o yaml | grep scc
```

Expected output:
```yaml
openshift.io/scc: restricted-v2
```

### If Custom SCC is Needed (Advanced)

If the virtual kubelet requires additional permissions (e.g., for node management), create a custom SCC:

```yaml
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: vk-flightctl-scc
allowHostDirVolumePlugin: false
allowHostIPC: false
allowHostNetwork: false
allowHostPID: false
allowHostPorts: false
allowPrivilegeEscalation: false
allowPrivilegedContainer: false
allowedCapabilities: []
defaultAddCapabilities: []
fsGroup:
  type: RunAsAny
groups: []
priority: null
readOnlyRootFilesystem: true
requiredDropCapabilities:
- ALL
runAsUser:
  type: MustRunAsRange
seLinuxContext:
  type: MustRunAs
supplementalGroups:
  type: RunAsAny
users:
- system:serviceaccount:codeco:vk-flightctl-provider
volumes:
- configMap
- downwardAPI
- emptyDir
- persistentVolumeClaim
- projected
- secret
```

Apply the SCC:
```bash
oc apply -f deploy/scc.yaml
```

## Troubleshooting

### Permission Denied Errors

If you see permission errors:

```bash
# Check SCC constraints
oc describe scc restricted-v2

# Check pod security context
oc get pod -n codeco -l app=vk-flightctl-provider -o jsonpath='{.items[0].spec.securityContext}'

# Check container security context
oc get pod -n codeco -l app=vk-flightctl-provider -o jsonpath='{.items[0].spec.containers[0].securityContext}'
```

### Random UID Issues

OpenShift assigns a random UID from the project's range. To check:

```bash
# Get project UID range
oc describe project codeco | grep uid-range

# Check actual UID the pod is using
oc exec -n codeco deployment/vk-flightctl-provider -- id
```

Expected output:
```
uid=1000660000(1000660000) gid=0(root) groups=0(root),1000660000
```

### Image Pull Issues

If deploying from quay.io:

```bash
# Create image pull secret if needed
oc create secret docker-registry quay-pull-secret \
  --docker-server=quay.io \
  --docker-username=YOUR_USERNAME \
  --docker-password=YOUR_PASSWORD \
  -n codeco

# Link to service account
oc secrets link vk-flightctl-provider quay-pull-secret --for=pull -n codeco
```

## Node Registration Issues

Virtual Kubelet needs permissions to register nodes. Verify RBAC:

```bash
# Check ClusterRole permissions
oc describe clusterrole vk-flightctl-provider

# Check ClusterRoleBinding
oc describe clusterrolebinding vk-flightctl-provider

# Test node permissions
oc auth can-i create nodes --as=system:serviceaccount:codeco:vk-flightctl-provider
```

## Network Policies (Optional)

If your cluster uses NetworkPolicies, allow egress to Flightctl API:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vk-flightctl-egress
  namespace: codeco
spec:
  podSelector:
    matchLabels:
      app: vk-flightctl-provider
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS to Flightctl API
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    ports:
    - protocol: TCP
      port: 443  # Kubernetes API
```

## Resource Quotas

Ensure the namespace has sufficient quota:

```bash
# Check resource quota
oc describe quota -n codeco

# If needed, request quota increase
oc create quota vk-flightctl-quota \
  --hard=cpu=1,memory=1Gi,pods=5 \
  -n codeco
```

## Monitoring

### Add Prometheus Annotations

Update deployment to expose metrics (if your app provides them):

```yaml
template:
  metadata:
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8080"
      prometheus.io/path: "/metrics"
```

### OpenShift Monitoring Integration

Enable monitoring in the namespace:

```bash
oc label namespace codeco openshift.io/cluster-monitoring=true
```

## Differences from Standard Kubernetes

| Feature | Kubernetes | OpenShift |
|---------|-----------|-----------|
| User ID | Static (e.g., 65532) | Random from namespace range |
| Group ID | Any | Always 0 (root group) |
| Default SCC | - | restricted-v2 |
| Image Registry | Any | Often internal registry |
| Route/Ingress | Ingress | Route (preferred) |
| Security | Pod Security Standards | Security Context Constraints |

## Production Checklist

- [ ] OAuth credentials stored in sealed secrets or external secret manager
- [ ] Image pulled from private registry with authentication
- [ ] Resource limits configured appropriately
- [ ] Network policies defined
- [ ] Monitoring and alerting configured
- [ ] Log aggregation enabled
- [ ] Backup strategy for configuration
- [ ] High availability considered (if needed)
- [ ] Update/rollout strategy defined
- [ ] Security scanning enabled for images
