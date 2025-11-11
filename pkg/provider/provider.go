package provider

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/raycarroll/vk-flightctl-provider/pkg/flightctl"
	"github.com/raycarroll/vk-flightctl-provider/pkg/models"
)

// Provider implements the Virtual Kubelet provider interface.
type Provider struct {
	nodeName string
	flightctl *flightctl.Client
	podManager *flightctl.PodManager

	// Pod tracking
	podMappings map[string]*models.PodDeviceMapping // podKey -> mapping
	mu          sync.RWMutex
}

// Config holds provider configuration.
type Config struct {
	NodeName           string
	FlightctlAPIURL    string
	FlightctlAuthToken string
	FlightctlInsecureTLS bool
}

// NewProvider creates a new Virtual Kubelet provider.
func NewProvider(cfg Config) (*Provider, error) {
	if cfg.NodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}

	// Create Flightctl client
	client, err := flightctl.NewClient(flightctl.Config{
		APIURL:      cfg.FlightctlAPIURL,
		AuthToken:   cfg.FlightctlAuthToken,
		InsecureTLS: cfg.FlightctlInsecureTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Flightctl client: %w", err)
	}

	return &Provider{
		nodeName:    cfg.NodeName,
		flightctl:   client,
		podManager:  flightctl.NewPodManager(client),
		podMappings: make(map[string]*models.PodDeviceMapping),
	}, nil
}

// PodLifecycleHandler interface implementation

// CreatePod deploys a pod to an edge device.
func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	// For minimal prototype, use a mock device ID
	// In full implementation, this would use DeploymentTarget selection
	deviceID := "mock-device-001"

	// Deploy to Flightctl
	if err := p.podManager.DeployPod(ctx, pod, deviceID); err != nil {
		return fmt.Errorf("deploying pod to device %s: %w", deviceID, err)
	}

	// Track mapping
	mapping := models.NewPodDeviceMapping(pod.Namespace, pod.Name, pod.UID, deviceID)
	p.podMappings[podKey] = mapping

	return nil
}

// UpdatePod updates a pod on an edge device.
func (p *Provider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
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
	p.mu.RLock()
	podKey := fmt.Sprintf("%s/%s", namespace, name)
	mapping := p.podMappings[podKey]
	p.mu.RUnlock()

	if mapping == nil {
		return nil, fmt.Errorf("pod not found")
	}

	// Get status from Flightctl
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       mapping.PodUID,
		},
	}

	status, err := p.podManager.GetPodStatus(ctx, pod, mapping.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("getting pod status: %w", err)
	}

	pod.Status = *status
	return pod, nil
}

// GetPods retrieves all pods managed by this provider.
func (p *Provider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pods := make([]*corev1.Pod, 0, len(p.podMappings))
	for podKey, mapping := range p.podMappings {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mapping.PodKey[:len(podKey)-len(mapping.PodKey)+1], // extract namespace
				Name:      podKey[len(podKey)-len(mapping.PodKey)+1:],         // extract name
				UID:       mapping.PodUID,
			},
		}

		// Get status from Flightctl
		status, err := p.podManager.GetPodStatus(ctx, pod, mapping.DeviceID)
		if err == nil {
			pod.Status = *status
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
func (p *Provider) NotifyNodeStatus(ctx context.Context, callback func(*corev1.Node)) error {
	// Minimal implementation - just call once
	node, err := p.GetNode(ctx)
	if err != nil {
		return err
	}
	callback(node)
	return nil
}

// GetNode returns the virtual node representing edge devices.
func (p *Provider) GetNode(ctx context.Context) (*corev1.Node, error) {
	// Minimal implementation with mock capacity
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.nodeName,
			Labels: map[string]string{
				"type":                     "virtual-kubelet",
				"kubernetes.io/role":       "agent",
				"kubernetes.io/hostname":   p.nodeName,
				"node.kubernetes.io/vk":    "flightctl",
			},
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
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
