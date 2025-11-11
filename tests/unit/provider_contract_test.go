package unit

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/raycarroll/vk-flightctl-provider/pkg/provider"
)

// TestPodLifecycleHandler_CreatePod tests the contract for CreatePod method.
// This test MUST FAIL until the provider implements PodLifecycleHandler.
func TestPodLifecycleHandler_CreatePod(t *testing.T) {
	// This will fail because provider.Provider doesn't exist yet
	var p provider.PodLifecycleHandler

	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	// Test CreatePod signature
	err := p.CreatePod(ctx, pod)
	if err == nil {
		t.Error("Expected error from unimplemented CreatePod, got nil")
	}
}

// TestPodLifecycleHandler_UpdatePod tests the contract for UpdatePod method.
func TestPodLifecycleHandler_UpdatePod(t *testing.T) {
	var p provider.PodLifecycleHandler

	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	err := p.UpdatePod(ctx, pod)
	if err == nil {
		t.Error("Expected error from unimplemented UpdatePod, got nil")
	}
}

// TestPodLifecycleHandler_DeletePod tests the contract for DeletePod method.
func TestPodLifecycleHandler_DeletePod(t *testing.T) {
	var p provider.PodLifecycleHandler

	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	err := p.DeletePod(ctx, pod)
	if err == nil {
		t.Error("Expected error from unimplemented DeletePod, got nil")
	}
}

// TestPodLifecycleHandler_GetPod tests the contract for GetPod method.
func TestPodLifecycleHandler_GetPod(t *testing.T) {
	var p provider.PodLifecycleHandler

	ctx := context.Background()

	pod, err := p.GetPod(ctx, "default", "test-pod")
	if pod != nil || err == nil {
		t.Error("Expected nil pod and error from unimplemented GetPod")
	}
}

// TestPodLifecycleHandler_GetPods tests the contract for GetPods method.
func TestPodLifecycleHandler_GetPods(t *testing.T) {
	var p provider.PodLifecycleHandler

	ctx := context.Background()

	pods, err := p.GetPods(ctx)
	if pods != nil || err == nil {
		t.Error("Expected nil pods and error from unimplemented GetPods")
	}
}

// TestPodLifecycleHandler_GetPodStatus tests the contract for GetPodStatus method.
func TestPodLifecycleHandler_GetPodStatus(t *testing.T) {
	var p provider.PodLifecycleHandler

	ctx := context.Background()

	status, err := p.GetPodStatus(ctx, "default", "test-pod")
	if status != nil || err == nil {
		t.Error("Expected nil status and error from unimplemented GetPodStatus")
	}
}
