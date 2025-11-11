package models

import "time"

// Fleet represents a logical grouping of edge devices.
type Fleet struct {
	// Identity
	ID     string
	Name   string
	Labels map[string]string

	// Metadata
	DeviceCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate checks if the fleet is valid.
func (f *Fleet) Validate() error {
	if f.ID == "" {
		return ErrInvalidFleet("fleet ID is required")
	}
	if f.Name == "" {
		return ErrInvalidFleet("fleet name is required")
	}
	if f.DeviceCount < 0 {
		return ErrInvalidFleet("device count cannot be negative")
	}
	return nil
}

// ErrInvalidFleet is returned when a fleet is invalid.
type ErrInvalidFleet string

func (e ErrInvalidFleet) Error() string {
	return "invalid fleet: " + string(e)
}
