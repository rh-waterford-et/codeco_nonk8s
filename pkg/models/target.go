package models

import (
	"fmt"
	"sort"
)

// DeploymentTarget represents the specification for targeting devices.
type DeploymentTarget struct {
	FleetID   *string           // If set, target devices in this fleet
	Selectors map[string]string // Label selectors (AND logic)
	DeviceID  *string           // If set, target specific device (overrides other fields)
}

// SelectDevice selects the best device from the candidate list.
// Algorithm:
// 1. Build candidate list (fleet + label filters)
// 2. Filter by ConnectionState=Connected and sufficient resources
// 3. Sort by available resources (descending)
// 4. Tie-break by fewest existing pods
func (dt *DeploymentTarget) SelectDevice(devices []*Device, podsByDevice map[string]int) (*Device, error) {
	if dt.DeviceID != nil {
		// Direct device targeting
		for _, d := range devices {
			if d.ID == *dt.DeviceID {
				if !d.IsReady() {
					return nil, fmt.Errorf("device %s is not ready", *dt.DeviceID)
				}
				return d, nil
			}
		}
		return nil, fmt.Errorf("device %s not found", *dt.DeviceID)
	}

	// Build candidate list
	var candidates []*Device
	for _, d := range devices {
		if !dt.matchesDevice(d) {
			continue
		}
		if !d.IsReady() {
			continue
		}
		candidates = append(candidates, d)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable device found")
	}

	// Sort by available resources, then by pod count
	sort.Slice(candidates, func(i, j int) bool {
		// Compare available CPU
		cpuI := candidates[i].Allocatable.CPU.Value()
		cpuJ := candidates[j].Allocatable.CPU.Value()
		if cpuI != cpuJ {
			return cpuI > cpuJ
		}

		// Tie-break by pod count
		podsI := podsByDevice[candidates[i].ID]
		podsJ := podsByDevice[candidates[j].ID]
		return podsI < podsJ
	})

	return candidates[0], nil
}

// matchesDevice checks if a device matches the target criteria.
func (dt *DeploymentTarget) matchesDevice(d *Device) bool {
	// Check fleet
	if dt.FleetID != nil && d.FleetID != *dt.FleetID {
		return false
	}

	// Check label selectors (AND logic)
	for key, value := range dt.Selectors {
		if d.Labels[key] != value {
			return false
		}
	}

	return true
}

// Validate checks if the deployment target is valid.
func (dt *DeploymentTarget) Validate() error {
	if dt.DeviceID == nil && dt.FleetID == nil && len(dt.Selectors) == 0 {
		return fmt.Errorf("at least one targeting field must be set")
	}
	return nil
}
