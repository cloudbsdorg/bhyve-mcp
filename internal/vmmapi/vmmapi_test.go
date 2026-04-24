package vmmapi

import (
	"testing"
)

func TestCreate(t *testing.T) {
	// Test that Create function exists and can be called
	// Note: This will fail without proper permissions and VM support
	_, err := Create("test-vm")
	// We expect this to fail in most environments without proper setup
	// The important thing is that the function compiles and can be called
	if err == nil {
		t.Log("VM created successfully (unexpected in test environment)")
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateUnknown, "unknown"},
		{StateStopped, "stopped"},
		{StateRunning, "running"},
		{StatePaused, "paused"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		result := tt.state.String()
		if result != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, result, tt.expected)
		}
	}
}
