package flightctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
)

// PodManager handles pod deployment operations via Flightctl API.
// Works directly with v1.Pod objects (no intermediate Workload abstraction).
type PodManager struct {
	client *Client
}

// NewPodManager creates a new pod manager.
func NewPodManager(client *Client) *PodManager {
	return &PodManager{client: client}
}

// DeployPod deploys a Kubernetes pod to a Flightctl device.
// Converts v1.Pod to Flightctl workload format and deploys.
func (pm *PodManager) DeployPod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	// Convert pod to Flightctl workload format
	workload := pm.podToFlightctlWorkload(pod, deviceID)

	// Marshal to JSON
	body, err := json.Marshal(workload)
	if err != nil {
		return fmt.Errorf("marshaling workload: %w", err)
	}

	// Send to Flightctl
	url := fmt.Sprintf("%s/api/v1/devices/%s/workloads", pm.client.baseURL, deviceID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating deploy request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+pm.client.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deploy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploy failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UpdatePod updates a pod on a device (simple replace strategy).
func (pm *PodManager) UpdatePod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	// Simple replace: delete then deploy
	_ = pm.DeletePod(ctx, pod, deviceID) // Ignore error if not exists
	return pm.DeployPod(ctx, pod, deviceID)
}

// DeletePod removes a pod from a device (idempotent).
func (pm *PodManager) DeletePod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	workloadID := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)
	url := fmt.Sprintf("%s/api/v1/devices/%s/workloads/%s", pm.client.baseURL, deviceID, workloadID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+pm.client.authToken)

	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	// Idempotent: 404 is OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetPodStatus retrieves pod status from Flightctl and maps to v1.PodStatus.
func (pm *PodManager) GetPodStatus(ctx context.Context, pod *corev1.Pod, deviceID string) (*corev1.PodStatus, error) {
	workloadID := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)
	url := fmt.Sprintf("%s/api/v1/devices/%s/workloads/%s/status", pm.client.baseURL, deviceID, workloadID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+pm.client.authToken)

	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pod not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request returned %d", resp.StatusCode)
	}

	// Parse Flightctl status response
	var flightctlStatus FlightctlWorkloadStatus
	if err := json.NewDecoder(resp.Body).Decode(&flightctlStatus); err != nil {
		return nil, fmt.Errorf("decoding status: %w", err)
	}

	// Map to v1.PodStatus
	return pm.flightctlStatusToPodStatus(&flightctlStatus), nil
}

// podToFlightctlWorkload converts a Kubernetes pod to Flightctl workload format.
func (pm *PodManager) podToFlightctlWorkload(pod *corev1.Pod, deviceID string) *FlightctlWorkload {
	containers := make([]FlightctlContainer, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		containers[i] = FlightctlContainer{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
			Args:    c.Args,
		}
	}

	return &FlightctlWorkload{
		ID:         fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		DeviceID:   deviceID,
		Namespace:  pod.Namespace,
		Name:       pod.Name,
		Containers: containers,
	}
}

// flightctlStatusToPodStatus maps Flightctl status to Kubernetes pod status.
func (pm *PodManager) flightctlStatusToPodStatus(status *FlightctlWorkloadStatus) *corev1.PodStatus {
	// Simple mapping - in real implementation, this would be more sophisticated
	phase := corev1.PodPending
	if status.State == "running" {
		phase = corev1.PodRunning
	} else if status.State == "failed" {
		phase = corev1.PodFailed
	} else if status.State == "succeeded" {
		phase = corev1.PodSucceeded
	}

	return &corev1.PodStatus{
		Phase:   phase,
		Message: status.Message,
	}
}

// FlightctlWorkload represents a workload in Flightctl format.
type FlightctlWorkload struct {
	ID         string                `json:"id"`
	DeviceID   string                `json:"deviceId"`
	Namespace  string                `json:"namespace"`
	Name       string                `json:"name"`
	Containers []FlightctlContainer  `json:"containers"`
}

// FlightctlContainer represents a container in Flightctl format.
type FlightctlContainer struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// FlightctlWorkloadStatus represents workload status from Flightctl.
type FlightctlWorkloadStatus struct {
	State   string `json:"state"` // running, pending, failed, succeeded
	Message string `json:"message,omitempty"`
}
