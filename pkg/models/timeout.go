package models

import (
	"context"
	"fmt"
	"time"
)

// TimeoutTracker tracks device disconnection timeouts for pod rescheduling.
type TimeoutTracker struct {
	DeviceID        string
	DisconnectedAt  time.Time
	TimeoutDuration time.Duration
	TimeoutAt       time.Time
	AffectedPods    []string // Pod keys (namespace/name)
	TimerCancelFunc context.CancelFunc
}

// NewTimeoutTracker creates a new timeout tracker.
func NewTimeoutTracker(deviceID string, duration time.Duration, affectedPods []string) (*TimeoutTracker, error) {
	if duration < time.Minute {
		return nil, fmt.Errorf("timeout duration must be >= 1 minute")
	}
	if duration > 30*time.Minute {
		return nil, fmt.Errorf("timeout duration must be <= 30 minutes")
	}

	now := time.Now()
	return &TimeoutTracker{
		DeviceID:        deviceID,
		DisconnectedAt:  now,
		TimeoutDuration: duration,
		TimeoutAt:       now.Add(duration),
		AffectedPods:    affectedPods,
	}, nil
}

// IsExpired checks if the timeout has been reached.
func (t *TimeoutTracker) IsExpired() bool {
	return time.Now().After(t.TimeoutAt)
}

// Cancel cancels the timeout (when device reconnects).
func (t *TimeoutTracker) Cancel() {
	if t.TimerCancelFunc != nil {
		t.TimerCancelFunc()
	}
}
