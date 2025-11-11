package models

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Device represents an edge device managed by Flightctl.
type Device struct {
	// Identity
	ID      string
	Name    string
	FleetID string
	Labels  map[string]string

	// Capacity
	Capacity    ResourceList
	Allocatable ResourceList

	// Status
	Status          DeviceStatus
	LastHeartbeat   time.Time
	ConnectionState ConnectionState

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ResourceList represents CPU and memory resources.
type ResourceList struct {
	CPU    resource.Quantity
	Memory resource.Quantity
}

// DeviceStatus represents the current state of a device.
type DeviceStatus struct {
	Phase   DevicePhase
	Message string
	Reason  string
}

// DevicePhase represents the phase of a device.
type DevicePhase string

const (
	DeviceReady    DevicePhase = "Ready"
	DeviceNotReady DevicePhase = "NotReady"
	DeviceUnknown  DevicePhase = "Unknown"
)

// ConnectionState represents the connectivity state of a device.
type ConnectionState string

const (
	Connected    ConnectionState = "Connected"
	Disconnected ConnectionState = "Disconnected"
	Unknown      ConnectionState = "Unknown"
)

// IsReady returns true if the device is in Ready phase.
func (d *Device) IsReady() bool {
	return d.Status.Phase == DeviceReady && d.ConnectionState == Connected
}

// HasSufficientResources checks if the device has enough resources for the given request.
func (d *Device) HasSufficientResources(cpu, memory resource.Quantity) bool {
	cpuAvail := d.Allocatable.CPU.DeepCopy()
	memAvail := d.Allocatable.Memory.DeepCopy()

	cpuAvail.Sub(cpu)
	memAvail.Sub(memory)

	return !cpuAvail.IsZero() && cpuAvail.Sign() >= 0 && !memAvail.IsZero() && memAvail.Sign() >= 0
}
