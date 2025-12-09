package provider

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/raycarroll/vk-flightctl-provider/pkg/flightctl"
	"github.com/raycarroll/vk-flightctl-provider/pkg/logger"
	"github.com/raycarroll/vk-flightctl-provider/pkg/models"
)

// Provider implements the Virtual Kubelet provider interface.
type Provider struct {
	nodeName   string
	flightctl  *flightctl.Client
	podManager *flightctl.PodManager

	// Pod tracking
	podMappings map[string]*models.PodDeviceMapping // podKey -> mapping
	mu          sync.RWMutex

	// Status reconciliation
	reconcileCtx    context.Context
	reconcileCancel context.CancelFunc
}

// Config holds provider configuration.
type Config struct {
	NodeName              string
	FlightctlAPIURL       string
	FlightctlClientID     string
	FlightctlClientSecret string
	FlightctlTokenURL     string
	FlightctlInsecureTLS  bool
}

// NewProvider creates a new Virtual Kubelet provider.
func NewProvider(cfg Config) (*Provider, error) {
	if cfg.NodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}

	// Create Flightctl client
	client, err := flightctl.NewClient(flightctl.Config{
		APIURL:       cfg.FlightctlAPIURL,
		ClientID:     cfg.FlightctlClientID,
		ClientSecret: cfg.FlightctlClientSecret,
		TokenURL:     cfg.FlightctlTokenURL,
		InsecureTLS:  cfg.FlightctlInsecureTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Flightctl client: %w", err)
	}

	// Create reconciliation context
	reconcileCtx, reconcileCancel := context.WithCancel(context.Background())

	p := &Provider{
		nodeName:        cfg.NodeName,
		flightctl:       client,
		podManager:      flightctl.NewPodManager(client),
		podMappings:     make(map[string]*models.PodDeviceMapping),
		reconcileCtx:    reconcileCtx,
		reconcileCancel: reconcileCancel,
	}

	// Start background status reconciliation loop
	go p.syncPodStatusLoop()

	return p, nil
}

// syncPodStatusLoop runs a background goroutine that periodically reconciles pod status with FlightCtl.
func (p *Provider) syncPodStatusLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.reconcileCtx.Done():
			logger.Info("Status reconciliation loop stopped")
			return
		case <-ticker.C:
			p.reconcilePodStatus()
		}
	}
}

// reconcilePodStatus fetches current status from FlightCtl for all tracked pods and updates cache.
func (p *Provider) reconcilePodStatus() {
	p.mu.RLock()
	// Create a snapshot of mappings to avoid holding lock during API calls
	mappings := make([]*models.PodDeviceMapping, 0, len(p.podMappings))
	for _, mapping := range p.podMappings {
		mappings = append(mappings, mapping)
	}
	p.mu.RUnlock()

	// Query status for each pod
	for _, mapping := range mappings {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mapping.Namespace,
				Name:      mapping.Name,
				UID:       mapping.PodUID,
			},
		}

		status, err := p.podManager.GetPodStatus(context.Background(), pod, mapping.DeviceID)
		if err != nil {
			logger.Error("Failed to get status for pod %s/%s: %v", mapping.Namespace, mapping.Name, err)
			continue
		}

		// Update cached status
		p.mu.Lock()
		if cachedMapping, exists := p.podMappings[mapping.PodKey]; exists {
			cachedMapping.Status = status
		}
		p.mu.Unlock()
	}
}

// Shutdown gracefully stops the provider and background goroutines.
func (p *Provider) Shutdown() {
	if p.reconcileCancel != nil {
		p.reconcileCancel()
	}
}

// selectDeviceForPod determines which FlightCtl device to deploy a pod to.
// Checks pod annotations for device/fleet selection:
// - flightctl.io/device-id: specific device ID
// - flightctl.io/fleet-id: fleet ID (TODO: implement fleet selection)
// Falls back to default device if no annotations present.
func (p *Provider) selectDeviceForPod(pod *corev1.Pod) (string, error) {
	const (
		deviceIDAnnotation = "flightctl.io/device-id"
		fleetIDAnnotation  = "flightctl.io/fleet-id"
		defaultDeviceID    = "d1k9ppdrurp23cfmj554f4rtt4f8uvo9mba4sp3in9arhli44ot0"
	)

	// Check for direct device ID annotation
	if deviceID, ok := pod.Annotations[deviceIDAnnotation]; ok && deviceID != "" {
		logger.Info("Pod %s/%s has device-id annotation: %s", pod.Namespace, pod.Name, deviceID)
		return deviceID, nil
	}

	// Check for fleet ID annotation
	if fleetID, ok := pod.Annotations[fleetIDAnnotation]; ok && fleetID != "" {
		logger.Info("Pod %s/%s has fleet-id annotation: %s", pod.Namespace, pod.Name, fleetID)
		// TODO: Implement fleet selection - query FlightCtl API for devices in fleet
		// For now, return error to indicate this is not yet implemented
		return "", fmt.Errorf("fleet-based device selection not yet implemented (fleet: %s)", fleetID)
	}

	// No annotations - use default device
	logger.Info("Pod %s/%s has no device/fleet annotations, using default device: %s",
		pod.Namespace, pod.Name, defaultDeviceID)
	return defaultDeviceID, nil
}

// PodLifecycleHandler interface implementation

// CreatePod deploys a pod to an edge device.
func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	logger.Info("Provider Create Pod %s", pod.Name)
	p.mu.Lock()
	defer p.mu.Unlock()

	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	// Select device from pod annotations or use default
	deviceID, err := p.selectDeviceForPod(pod)
	if err != nil {
		return fmt.Errorf("selecting device for pod: %w", err)
	}

	logger.Info("Deploying pod %s to device %s", podKey, deviceID)

	// Deploy to Flightctl
	if err := p.podManager.DeployPod(ctx, pod, deviceID); err != nil {
		return fmt.Errorf("deploying pod to device %s: %w", deviceID, err)
	}

	// Track mapping
	mapping := models.NewPodDeviceMapping(pod.Namespace, pod.Name, pod.UID, deviceID)

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

	p.podMappings[podKey] = mapping

	logger.Info("Pod %s created with initial Pending status", podKey)
	return nil
}

// UpdatePod updates a pod on an edge device.
func (p *Provider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	logger.Info("Provider Update Pod %s", pod.Name)
	p.mu.RLock()
	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	mapping := p.podMappings[podKey]
	p.mu.RUnlock()

	if mapping == nil {
		return fmt.Errorf("pod %s not found", podKey)
	}

	return p.podManager.UpdatePod(ctx, pod, mapping.DeviceID)
}

// DeletePod removes a pod from an edge device.
func (p *Provider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	logger.Info("Provider Delete Pod %s", pod.Name)
	p.mu.Lock()
	defer p.mu.Unlock()

	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	mapping := p.podMappings[podKey]

	if mapping == nil {
		// Already deleted (idempotent)
		return nil
	}

	// Delete from Flightctl
	if err := p.podManager.DeletePod(ctx, pod, mapping.DeviceID); err != nil {
		return fmt.Errorf("deleting pod from device: %w", err)
	}

	// Remove mapping
	delete(p.podMappings, podKey)

	return nil
}

// GetPod retrieves a pod's current status.
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	logger.Debug("Provider Get Pod %s", name)
	p.mu.RLock()
	podKey := fmt.Sprintf("%s/%s", namespace, name)
	mapping := p.podMappings[podKey]
	p.mu.RUnlock()

	if mapping == nil {
		return nil, fmt.Errorf("pod not found")
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       mapping.PodUID,
		},
	}

	// Use cached status if available
	p.mu.RLock()
	cachedStatus := mapping.Status
	p.mu.RUnlock()

	if cachedStatus != nil {
		pod.Status = *cachedStatus
		return pod, nil
	}

	// Fallback: query FlightCtl if no cached status (shouldn't happen after reconciliation starts)
	status, err := p.podManager.GetPodStatus(ctx, pod, mapping.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("getting pod status: %w", err)
	}

	// Update cache
	p.mu.Lock()
	mapping.Status = status
	p.mu.Unlock()

	pod.Status = *status
	return pod, nil
}

// GetPods retrieves all pods managed by this provider.
func (p *Provider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	logger.Debug("Provider Get Pods")
	p.mu.RLock()
	defer p.mu.RUnlock()

	pods := make([]*corev1.Pod, 0, len(p.podMappings))
	for _, mapping := range p.podMappings {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mapping.Namespace,
				Name:      mapping.Name,
				UID:       mapping.PodUID,
			},
		}

		// Use cached status if available
		if mapping.Status != nil {
			pod.Status = *mapping.Status
		}

		pods = append(pods, pod)
	}

	return pods, nil
}

// GetPodStatus retrieves just the status of a pod.
func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// NodeProvider interface implementation

// Ping checks provider health.
func (p *Provider) Ping(ctx context.Context) error {
	return p.flightctl.Ping(ctx)
}

// NotifyNodeStatus registers a node status callback.
// This method should be non-blocking and call the callback whenever the node status changes.
func (p *Provider) NotifyNodeStatus(ctx context.Context, callback func(*corev1.Node)) {
	// For now, just call once with initial node status
	// In a full implementation, this would monitor edge device status
	// and call the callback whenever capacity or conditions change
	node, err := p.GetNode(ctx)
	if err != nil {
		// Log error but don't block - NotifyNodeStatus should not return errors
		logger.Error("Error getting node status: %v", err)
		return
	}
	callback(node)
}

// GetNode returns the virtual node representing edge devices.
func (p *Provider) GetNode(ctx context.Context) (*corev1.Node, error) {
	// Minimal implementation with mock capacity
	logger.Debug("Getting Node: %s", p.nodeName)
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.nodeName,
			Labels: map[string]string{
				"type":                   "virtual-kubelet",
				"kubernetes.io/role":     "agent",
				"kubernetes.io/hostname": p.nodeName,
				"node.kubernetes.io/vk":  "flightctl",
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
				"node.kubernetes.io/exclude-from-external-load-balancers": "true",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "vkubelet-flightctl",
					Value:  "true",
					Effect: "NoSchedule",
				},
			},
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
					Reason:             "KubeletReady",
					Message:            "kubelet is ready.",
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
				corev1.ResourcePods:   resource.MustParse("100"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
				corev1.ResourcePods:   resource.MustParse("100"),
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:  "vk-flightctl-v1.0.0",
				Architecture:    "amd64",
				OperatingSystem: "Linux",
			},
		},
	}

	return node, nil
}

// Additional nodeutil.Provider interface methods

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *Provider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	// TODO: Implement log streaming from edge devices via Flightctl
	return nil, fmt.Errorf("GetContainerLogs not yet implemented")
}

// RunInContainer executes a command in a container in the pod.
func (p *Provider) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	// TODO: Implement command execution on edge devices via Flightctl
	return fmt.Errorf("RunInContainer not yet implemented")
}

// AttachToContainer attaches to the executing process of a container in the pod.
func (p *Provider) AttachToContainer(ctx context.Context, namespace, podName, containerName string, attach api.AttachIO) error {
	// TODO: Implement container attach via Flightctl
	return fmt.Errorf("AttachToContainer not yet implemented")
}

// GetStatsSummary gets the stats for the node, including running pods.
func (p *Provider) GetStatsSummary(ctx context.Context) (*statsv1alpha1.Summary, error) {
	// TODO: Aggregate stats from edge devices via Flightctl
	// For now, return minimal stats
	return &statsv1alpha1.Summary{
		Node: statsv1alpha1.NodeStats{
			NodeName: p.nodeName,
		},
		Pods: []statsv1alpha1.PodStats{},
	}, nil
}

// GetMetricsResource gets the metrics for the node, including running pods.
func (p *Provider) GetMetricsResource(ctx context.Context) ([]*dto.MetricFamily, error) {
	// TODO: Aggregate metrics from edge devices via Flightctl
	// For now, return empty metrics
	return []*dto.MetricFamily{}, nil
}

// PortForward forwards a local port to a port on the pod.
func (p *Provider) PortForward(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error {
	// TODO: Implement port forwarding to edge devices via Flightctl
	return fmt.Errorf("PortForward not yet implemented")
}
