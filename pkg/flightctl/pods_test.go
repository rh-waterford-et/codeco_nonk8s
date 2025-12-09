package flightctl

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertPodToDockerCompose(t *testing.T) {
	// Create a sample Kubernetes Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.21",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
						{
							ContainerPort: 443,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "NGINX_HOST",
							Value: "example.com",
						},
						{
							Name:  "NGINX_PORT",
							Value: "80",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "html-volume",
							MountPath: "/usr/share/nginx/html",
							ReadOnly:  false,
						},
						{
							Name:      "config-volume",
							MountPath: "/etc/nginx/conf.d",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "html-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "nginx-config",
							},
						},
					},
				},
			},
		},
	}

	// Convert to Docker Compose
	composeYAML := convertPodToDockerCompose(pod)

	// Print the output for manual inspection
	t.Logf("Generated Docker Compose:\n%s", composeYAML)

	// Basic validation
	if composeYAML == "" {
		t.Error("Expected non-empty Docker Compose output")
	}

	// Check for key elements
	// Note: volumes and resources are currently commented out in the implementation
	expectedStrings := []string{
		"version: '3.8'",
		"services:",
		"nginx:",
		"image: nginx:1.21",
		"ports:",
		"80:80",
		"443:443",
		"environment:",
		"NGINX_HOST=example.com",
		"NGINX_PORT=80",
		"restart: unless-stopped",
	}

	for _, expected := range expectedStrings {
		if !containsString(composeYAML, expected) {
			t.Errorf("Expected Docker Compose to contain '%s', but it didn't", expected)
		}
	}
}

func TestConvertPodToDockerCompose_MultiContainer(t *testing.T) {
	// Test a pod with multiple containers (sidecar pattern)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-with-sidecar",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "myapp:v1.0",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080},
					},
				},
				{
					Name:  "logging-sidecar",
					Image: "fluent/fluentd:v1.14",
					Args:  []string{"-c", "/fluentd/etc/fluent.conf"},
				},
			},
		},
	}

	composeYAML := convertPodToDockerCompose(pod)
	t.Logf("Multi-container Docker Compose:\n%s", composeYAML)

	// Verify both services exist
	if !containsString(composeYAML, "app:") {
		t.Error("Expected 'app' service in Docker Compose")
	}
	if !containsString(composeYAML, "logging-sidecar:") {
		t.Error("Expected 'logging-sidecar' service in Docker Compose")
	}
}

func TestConvertPodToDockerCompose_EmptyPod(t *testing.T) {
	// Test with nil pod
	composeYAML := convertPodToDockerCompose(nil)
	if composeYAML != "" {
		t.Error("Expected empty string for nil pod")
	}

	// Test with pod with no containers
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}
	composeYAML = convertPodToDockerCompose(pod)
	if composeYAML != "" {
		t.Error("Expected empty string for pod with no containers")
	}
}

// Helper function
func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) >= len(needle) &&
		indexOfString(haystack, needle) >= 0)
}

func indexOfString(haystack, needle string) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
