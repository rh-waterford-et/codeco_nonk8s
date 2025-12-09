package flightctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/raycarroll/vk-flightctl-provider/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// Fetches the existing Device, adds the pod as a new application, and updates the Device.
func (pm *PodManager) DeployPod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	logger.Info("PodManager.DeployPod() for pod %s on device %s", pod.Name, deviceID)

	// Step 1: Get the existing Device resource
	logger.Debug("Retrieve Device info from flightctl")
	device, err := pm.getDevice(ctx, deviceID)
	if err != nil {
		logger.Error("getting device %s: %s", deviceID, err.Error())
		return fmt.Errorf("getting device %s: %w", deviceID, err)
	}

	// Step 2: Convert pod to Flightctl Application
	logger.Debug("Converting Pod to FlightCTL App Spec")
	newApp := pm.podToFlightctlApplication(pod)

	// Step 3: Check if application already exists and remove it (update scenario)
	existingApps := make([]FlightctlApplication, 0, len(device.Spec.Applications))
	for _, app := range device.Spec.Applications {
		if app.Name != newApp.Name {
			existingApps = append(existingApps, app)
		}
	}

	// Step 4: Add the new application
	existingApps = append(existingApps, newApp)
	device.Spec.Applications = existingApps
	device.Status = nil

	logger.Info("Updated device with %d applications", len(device.Spec.Applications))

	// Step 5: Update the Device resource
	return pm.updateDevice(ctx, deviceID, device)
}

// UpdatePod updates a pod on a device (simple replace strategy).
func (pm *PodManager) UpdatePod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	// Simple replace: delete then deploy
	logger.Info("PodManager.UpdatePod() for pod %s on device %s", pod.Name, deviceID)
	_ = pm.DeletePod(ctx, pod, deviceID) // Ignore error if not exists
	return pm.DeployPod(ctx, pod, deviceID)
}

// DeletePod removes a pod from a device by removing its application from the Device spec.
// This operation is idempotent - if the application doesn't exist, no error is returned.
func (pm *PodManager) DeletePod(ctx context.Context, pod *corev1.Pod, deviceID string) error {
	logger.Info("PodManager.DeletePod() for pod %s on device %s", pod.Name, deviceID)

	// Step 1: Get the existing Device resource
	device, err := pm.getDevice(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("getting device %s: %w", deviceID, err)
	}

	// Step 2: Generate the application name that would have been created
	appName := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)

	// Step 3: Filter out the application to delete
	updatedApps := make([]FlightctlApplication, 0, len(device.Spec.Applications))
	found := false
	for _, app := range device.Spec.Applications {
		if app.Name != appName {
			updatedApps = append(updatedApps, app)
		} else {
			found = true
		}
	}

	// If application wasn't found, that's OK (idempotent)
	if !found {
		logger.Info("Application %s not found on device %s (already deleted)", appName, deviceID)
		return nil
	}

	// Step 4: Update the device with the filtered application list
	device.Spec.Applications = updatedApps
	device.Status = nil
	logger.Info("Removing application %s from device %s (%d applications remaining)", appName, deviceID, len(updatedApps))

	return pm.updateDevice(ctx, deviceID, device)
}

// GetPodStatus retrieves pod status from Flightctl Device resource and maps to v1.PodStatus.
func (pm *PodManager) GetPodStatus(ctx context.Context, pod *corev1.Pod, deviceID string) (*corev1.PodStatus, error) {
	appName := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)

	// Get the Device resource
	device, err := pm.getDevice(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("getting device %s: %w", deviceID, err)
	}

	// Check if the application exists in the Device spec
	appExists := false
	for _, app := range device.Spec.Applications {
		if app.Name == appName {
			appExists = true
			break
		}
	}

	if !appExists {
		return nil, fmt.Errorf("application %s not found on device %s", appName, deviceID)
	}

	// Check Device status for actual runtime status
	if device.Status != nil && len(device.Status.Applications) > 0 {
		for _, appStatus := range device.Status.Applications {
			if appStatus.Name == appName {
				// Found runtime status - map to Kubernetes pod status
				return pm.mapFlightctlStatusToPodStatus(&appStatus), nil
			}
		}
	}

	// No runtime status available yet - application is in spec but not yet running
	// Return Pending status
	return &corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
				Reason: "ApplicationDeployed",
			},
		},
	}, nil
}

// mapFlightctlStatusToPodStatus maps FlightCtl application status to Kubernetes pod status.
func (pm *PodManager) mapFlightctlStatusToPodStatus(appStatus *FlightctlApplicationStatus) *corev1.PodStatus {
	var phase corev1.PodPhase
	var conditions []corev1.PodCondition

	// Map FlightCtl status to Kubernetes phase
	// Common FlightCtl statuses: running, pending, starting, failed, stopped, completed, error
	switch strings.ToLower(appStatus.Status) {
	case "running":
		phase = corev1.PodRunning
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "ApplicationRunning",
				Message:            appStatus.Summary,
			},
		}

	case "pending", "starting":
		phase = corev1.PodPending
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "ApplicationStarting",
				Message:            appStatus.Summary,
			},
		}

	case "failed", "error":
		phase = corev1.PodFailed
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "ApplicationFailed",
				Message:            appStatus.Summary,
			},
		}

	case "completed", "succeeded":
		phase = corev1.PodSucceeded
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "ApplicationCompleted",
				Message:            appStatus.Summary,
			},
		}

	case "stopped":
		// Stopped but not failed - map to Succeeded
		phase = corev1.PodSucceeded
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "ApplicationStopped",
				Message:            appStatus.Summary,
			},
		}

	default:
		// Unknown status - default to Pending
		phase = corev1.PodPending
		conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "UnknownStatus",
				Message:            fmt.Sprintf("Unknown application status: %s - %s", appStatus.Status, appStatus.Summary),
			},
		}
	}

	return &corev1.PodStatus{
		Phase:      phase,
		Conditions: conditions,
		Message:    appStatus.Summary,
	}
}

// getDevice retrieves the current Device resource from FlightCtl API.
func (pm *PodManager) getDevice(ctx context.Context, deviceID string) (*FlightctlDevice, error) {
	url := fmt.Sprintf("%s/api/v1/devices/%s", pm.client.baseURL, deviceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GET request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		logger.Error("GET request failed: %v", err)
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("GET device failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("GET device failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var device FlightctlDevice
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		logger.Error("decoding device: %s", err.Error())
		return nil, fmt.Errorf("decoding device: %w", err)
	}

	return &device, nil
}

// updateDevice updates a Device resource via FlightCtl API (PUT).
func (pm *PodManager) updateDevice(ctx context.Context, deviceID string, device *FlightctlDevice) error {
	url := fmt.Sprintf("%s/api/v1/devices/%s", pm.client.baseURL, deviceID)

	body, err := json.Marshal(device)
	if err != nil {
		return fmt.Errorf("marshaling device: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating PUT request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	logger.Debug("Updating device %s with payload:\n%s", deviceID, string(body))

	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("update device failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("update device failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	logger.Info("Successfully updated device %s", deviceID)
	return nil
}

// convertPodToDockerCompose converts a Kubernetes Pod to Docker Compose YAML format.
// This creates a docker-compose.yml compatible string that can be deployed via FlightCtl.
func convertPodToDockerCompose(pod *corev1.Pod) string {
	if pod == nil || len(pod.Spec.Containers) == 0 {
		return ""
	}

	var compose strings.Builder
	//compose.WriteString("content: |\n")
	compose.WriteString(" version: '3.8'\n")
	compose.WriteString(" services:\n")

	// Convert each container to a service
	for _, container := range pod.Spec.Containers {
		compose.WriteString(fmt.Sprintf("  %s:\n", sanitizeServiceName(container.Name)))

		// Image
		compose.WriteString(fmt.Sprintf("    image: %s\n", container.Image))

		// Command (entrypoint in Docker Compose)
		if len(container.Command) > 0 {
			compose.WriteString("    entrypoint:\n")
			for _, cmd := range container.Command {
				compose.WriteString(fmt.Sprintf("      - %s\n", cmd))
			}
		}

		// Args (command in Docker Compose)
		if len(container.Args) > 0 {
			compose.WriteString("    command:\n")
			for _, arg := range container.Args {
				compose.WriteString(fmt.Sprintf("      - %s\n", arg))
			}
		}

		// Environment variables
		if len(container.Env) > 0 {
			compose.WriteString("    environment:\n")
			for _, env := range container.Env {
				if env.Value != "" {
					// Direct value
					compose.WriteString(fmt.Sprintf("      - %s=%s\n", env.Name, env.Value))
				} else if env.ValueFrom != nil {
					// For now, we'll add a placeholder comment for complex env sources
					compose.WriteString(fmt.Sprintf("      # %s: (from secret/configmap)\n", env.Name))
				}
			}
		}

		// Ports
		if len(container.Ports) > 0 {
			compose.WriteString("    ports:\n")
			for _, port := range container.Ports {
				if port.ContainerPort > 0 {
					// Map container port to same host port
					compose.WriteString(fmt.Sprintf("      - \"%d:%d\"\n", port.ContainerPort, port.ContainerPort))
				}
			}
		}

		// Volume mounts
		// if len(container.VolumeMounts) > 0 {
		// 	compose.WriteString("    volumes:\n")
		// 	for _, mount := range container.VolumeMounts {
		// 		readOnly := ""
		// 		if mount.ReadOnly {
		// 			readOnly = ":ro"
		// 		}
		// 		// Create named volume or bind mount
		// 		compose.WriteString(fmt.Sprintf("      - %s:%s%s\n",
		// 			sanitizeVolumeName(mount.Name), mount.MountPath, readOnly))
		// 	}
		// }

		// // Resource limits and requests
		// if container.Resources.Limits != nil || container.Resources.Requests != nil {
		// 	compose.WriteString("    deploy:\n")
		// 	compose.WriteString("      resources:\n")

		// 	// Limits
		// 	if container.Resources.Limits != nil {
		// 		compose.WriteString("        limits:\n")
		// 		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
		// 			compose.WriteString(fmt.Sprintf("          cpus: '%s'\n", cpu.String()))
		// 		}
		// 		if mem := container.Resources.Limits.Memory(); mem != nil {
		// 			compose.WriteString(fmt.Sprintf("          memory: %s\n", mem.String()))
		// 		}
		// 	}

		// 	// Requests (reservations in Docker Compose)
		// 	if container.Resources.Requests != nil {
		// 		compose.WriteString("        reservations:\n")
		// 		if cpu := container.Resources.Requests.Cpu(); cpu != nil {
		// 			compose.WriteString(fmt.Sprintf("          cpus: '%s'\n", cpu.String()))
		// 		}
		// 		if mem := container.Resources.Requests.Memory(); mem != nil {
		// 			compose.WriteString(fmt.Sprintf("          memory: %s\n", mem.String()))
		// 		}
		// 	}
		// }

		// Restart policy
		restartPolicy := "unless-stopped"
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			restartPolicy = "no"
		} else if pod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure {
			restartPolicy = "on-failure"
		}
		compose.WriteString(fmt.Sprintf("    restart: %s\n", restartPolicy))

		compose.WriteString("\n")
	}

	// Define volumes section if there are any volumes
	// if len(pod.Spec.Volumes) > 0 {
	// 	compose.WriteString("volumes:\n")
	// 	for _, vol := range pod.Spec.Volumes {
	// 		volumeName := sanitizeVolumeName(vol.Name)

	// 		if vol.HostPath != nil {
	// 			// Host path volumes are handled as bind mounts in the service definition
	// 			compose.WriteString(fmt.Sprintf("  # %s: host path %s\n", volumeName, vol.HostPath.Path))
	// 		} else if vol.EmptyDir != nil {
	// 			// Empty dir becomes a named volume
	// 			compose.WriteString(fmt.Sprintf("  %s:\n", volumeName))
	// 		} else if vol.ConfigMap != nil {
	// 			// ConfigMap - would need to be pre-created on the device
	// 			compose.WriteString(fmt.Sprintf("  # %s: from configmap %s\n", volumeName, vol.ConfigMap.Name))
	// 		} else if vol.Secret != nil {
	// 			// Secret - would need to be pre-created on the device
	// 			compose.WriteString(fmt.Sprintf("  # %s: from secret %s\n", volumeName, vol.Secret.SecretName))
	// 		} else {
	// 			// Generic named volume
	// 			compose.WriteString(fmt.Sprintf("  %s:\n", volumeName))
	// 		}
	// 	}
	// }

	return compose.String()
}

// sanitizeServiceName converts a Kubernetes container name to a valid Docker Compose service name.
func sanitizeServiceName(name string) string {
	// Docker Compose service names should be lowercase alphanumeric with underscores/hyphens
	return strings.ReplaceAll(strings.ToLower(name), ".", "-")
}

// sanitizeVolumeName converts a Kubernetes volume name to a valid Docker Compose volume name.
func sanitizeVolumeName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), ".", "-")
}

// podToFlightctlApplication converts a Kubernetes pod to a FlightCtl Application.
// Uses the first container's image and creates an application entry.
func (pm *PodManager) podToFlightctlApplication(pod *corev1.Pod) FlightctlApplication {
	// Use pod name as application name
	appName := fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)

	// Use the first container's image (most pods have a single primary container)
	//image := "docker.io/library/busybox:latest" // default fallback
	//if len(pod.Spec.Containers) > 0 {
	//	image = pod.Spec.Containers[0].Image
	//}

	//TODO: STEP2: Convert the pod to a docker compose format and add inline
	logger.Debug("Creating Inline Content Section")
	var inlineContent InlineContent
	var inlineContentArray []InlineContent

	inlineContent.Content = convertPodToDockerCompose(pod)
	inlineContent.Path = "podman-compose.yaml"
	inlineContentArray = append(inlineContentArray, inlineContent)

	jsonBytes, err := json.MarshalIndent(inlineContent, "", "  ")
	if err != nil {
		logger.Error("Error marshaling: %v", err)
	} else {
		logger.Debug("PodToCompose:\n%s", string(jsonBytes))
	}

	return FlightctlApplication{
		Name: appName,
		//Image:   image,
		AppType: "compose", // Default to compose type
		Inline:  inlineContentArray,
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

// FlightctlDevice represents a complete Device resource in Flightctl API format.
type FlightctlDevice struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   FlightctlDeviceMetadata `json:"metadata"`
	Spec       FlightctlDeviceSpec     `json:"spec"`
	Status     *FlightctlDeviceStatus  `json:"status,omitempty"`
}

// FlightctlDeviceMetadata represents the metadata section of a Device.
type FlightctlDeviceMetadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// FlightctlDeviceSpec represents the spec section of a Device.
type FlightctlDeviceSpec struct {
	Systemd      *FlightctlSystemdConfig `json:"systemd,omitempty"`
	Applications []FlightctlApplication  `json:"applications,omitempty"`
}

// FlightctlDeviceStatus represents the status section of a Device.
type FlightctlDeviceStatus struct {
	Applications []FlightctlApplicationStatus `json:"applications,omitempty"`
	Conditions   []FlightctlCondition         `json:"conditions,omitempty"`
}

// FlightctlApplicationStatus represents the runtime status of an application on a device.
type FlightctlApplicationStatus struct {
	Name    string `json:"name"`              // Application name
	Status  string `json:"status"`            // running, pending, failed, stopped, starting, etc.
	Summary string `json:"summary,omitempty"` // Human-readable summary
}

// FlightctlCondition represents a condition in the Device status.
type FlightctlCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// FlightctlSystemdConfig represents systemd configuration in Device spec.
type FlightctlSystemdConfig struct {
	MatchPatterns []string `json:"matchPatterns,omitempty"`
}

// FlightctlApplication represents an application in the Device applications list.
type FlightctlApplication struct {
	Name string `json:"name"`
	//Image   string          `json:"image"`
	AppType string          `json:"appType"` // "compose", "pod", etc.
	Inline  []InlineContent `json:"inline"`
}

type InlineContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FlightctlWorkload represents a workload in Flightctl format (deprecated, use Device+Applications instead).
type FlightctlWorkload struct {
	ID         string               `json:"id"`
	DeviceID   string               `json:"deviceId"`
	Namespace  string               `json:"namespace"`
	Name       string               `json:"name"`
	Containers []FlightctlContainer `json:"containers"`
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
