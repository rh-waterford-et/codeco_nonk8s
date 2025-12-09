# VK-Flightctl Provider Deployment

This directory contains Kubernetes manifests for deploying the Virtual Kubelet Flightctl Provider.

> **📘 OpenShift Users:** See [OPENSHIFT.md](OPENSHIFT.md) for OpenShift-specific deployment instructions and security considerations.

## Prerequisites

- Kubernetes cluster (v1.20+)
- `kubectl` configured with cluster access
- Docker or Podman for building the image
- Flightctl OAuth 2.0 credentials (client ID and client secret)

## Build the Container Image

From the repository root:

```bash
# Build the image
docker build -t vk-flightctl-provider:latest .

# Or with Podman
podman build -t vk-flightctl-provider:latest .

# Tag for your registry (optional)
docker tag vk-flightctl-provider:latest your-registry/vk-flightctl-provider:latest

# Push to registry (optional)
docker push your-registry/vk-flightctl-provider:latest
```

## Configuration

### 1. Update ConfigMap (Optional)

Edit `configmap.yaml` to customize:
- `flightctl-api-url`: Flightctl API endpoint
- `flightctl-token-url`: OAuth 2.0 token endpoint
- `flightctl-insecure-tls`: Set to "true" for development (not recommended for production)

### 2. Create Secret with OAuth Credentials

**Option A: Using kubectl (Recommended)**

```bash
kubectl create secret generic vk-flightctl-oauth \
  --from-literal=client-id=YOUR_CLIENT_ID \
  --from-literal=client-secret=YOUR_CLIENT_SECRET \
  --namespace=default
```

**Option B: Edit secret.yaml**

Edit `secret.yaml` and replace `YOUR_CLIENT_ID` and `YOUR_CLIENT_SECRET` with your actual credentials:

```yaml
stringData:
  client-id: "codeco"
  client-secret: "KWv1Hv11akMk3UKYpCOTseSXwpyaH0tK"
```

⚠️ **Warning**: Do not commit actual credentials to version control!

## Deployment

### Using kubectl

```bash
# Deploy all resources
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f deployment.yaml

# Or apply all at once
kubectl apply -f .
```

### Using Kustomize

```bash
# Deploy with kustomize
kubectl apply -k .

# Or with kubectl kustomize
kubectl kustomize . | kubectl apply -f -
```

## Verify Deployment

```bash
# Check pod status
kubectl get pods -l app=vk-flightctl-provider

# View logs
kubectl logs -l app=vk-flightctl-provider -f

# Check if virtual node is registered
kubectl get nodes
```

Expected output should show a node named `vk-flightctl-node` (or the value of `NODE_NAME` env var).

## Troubleshooting

### Pod fails to start

```bash
# Check pod events
kubectl describe pod -l app=vk-flightctl-provider

# Check logs
kubectl logs -l app=vk-flightctl-provider --tail=100
```

### Common issues

1. **OAuth authentication failure**
   - Verify client ID and secret are correct
   - Check token URL is accessible from the cluster
   - Ensure Flightctl OAuth server is running

2. **Cannot connect to Flightctl API**
   - Verify API URL in ConfigMap
   - Check network connectivity from cluster to Flightctl
   - Review TLS settings (insecure-tls)

3. **Missing permissions**
   - Ensure RBAC resources are created
   - Check ServiceAccount is bound to ClusterRole

## Update Deployment

### Update image

```bash
# Update image in deployment
kubectl set image deployment/vk-flightctl-provider \
  vk-flightctl-provider=vk-flightctl-provider:new-tag

# Or edit deployment directly
kubectl edit deployment vk-flightctl-provider
```

### Update configuration

```bash
# Update ConfigMap
kubectl edit configmap vk-flightctl-config

# Restart deployment to pick up changes
kubectl rollout restart deployment/vk-flightctl-provider
```

### Update OAuth credentials

```bash
# Delete and recreate secret
kubectl delete secret vk-flightctl-oauth
kubectl create secret generic vk-flightctl-oauth \
  --from-literal=client-id=NEW_CLIENT_ID \
  --from-literal=client-secret=NEW_CLIENT_SECRET

# Restart deployment
kubectl rollout restart deployment/vk-flightctl-provider
```

## Uninstall

```bash
# Delete all resources
kubectl delete -f .

# Or with kustomize
kubectl delete -k .
```

## Production Recommendations

1. **Use a private image registry** - Push the image to a private registry
2. **Use proper secrets management** - Consider using:
   - Kubernetes External Secrets Operator
   - HashiCorp Vault
   - AWS Secrets Manager / Azure Key Vault / GCP Secret Manager
3. **Enable resource limits** - Already configured in deployment.yaml
4. **Enable monitoring** - Add Prometheus metrics and health checks
5. **Use TLS** - Set `flightctl-insecure-tls: "false"` in production
6. **Network policies** - Restrict network access to Flightctl API
7. **Pod security** - Review and adjust securityContext settings
8. **Namespace isolation** - Deploy in a dedicated namespace

## Architecture

```
┌─────────────────────────────────────────┐
│ Kubernetes Cluster                      │
│                                         │
│  ┌────────────────────────────────┐    │
│  │ VK-Flightctl Provider Pod      │    │
│  │                                │    │
│  │ ┌────────────────────────────┐ │    │
│  │ │ Virtual Kubelet            │ │    │
│  │ │ - Registers virtual node   │ │    │
│  │ │ - Manages pod lifecycle    │ │    │
│  │ └────────────┬───────────────┘ │    │
│  │              │                 │    │
│  │ ┌────────────▼───────────────┐ │    │
│  │ │ Flightctl Client           │ │    │
│  │ │ - OAuth 2.0 auth           │ │    │
│  │ │ - Pod deployment to edge   │ │    │
│  │ └────────────┬───────────────┘ │    │
│  └──────────────┼─────────────────┘    │
│                 │                       │
└─────────────────┼───────────────────────┘
                  │ HTTPS + OAuth 2.0
                  ▼
         ┌─────────────────┐
         │ Flightctl API   │
         │                 │
         │ - Device mgmt   │
         │ - Workload ctrl │
         └────────┬────────┘
                  │
         ┌────────▼────────┐
         │ Edge Devices    │
         │ (Flightctl)     │
         └─────────────────┘
```
