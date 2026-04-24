package net

import (
	"fmt"
	"os/exec"
	"strings"
)

// SwitchInfo holds information about a virtual switch
type SwitchInfo struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Interfaces []string `json:"interfaces"`
}

// Manager manages network switches and bridges
type Manager struct {
	defaultBridge string
}

// NewManager creates a new network manager
func NewManager(defaultBridge string) *Manager {
	return &Manager{
		defaultBridge: defaultBridge,
	}
}

// CreateSwitch creates a virtual switch (bridge)
func (m *Manager) CreateSwitch(name string, physicalInterface string) error {
	// Create bridge interface
	cmd := exec.Command("ifconfig", name, "create")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	// Add physical interface to bridge if specified
	if physicalInterface != "" {
		cmd = exec.Command("ifconfig", name, "addm", physicalInterface)
		if err := cmd.Run(); err != nil {
			// Clean up bridge
			exec.Command("ifconfig", name, "destroy").Run()
			return fmt.Errorf("failed to add interface to bridge: %w", err)
		}

		// Bring up the physical interface
		cmd = exec.Command("ifconfig", physicalInterface, "up")
		cmd.Run()
	}

	// Bring up the bridge
	cmd = exec.Command("ifconfig", name, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up bridge: %w", err)
	}

	return nil
}

// DeleteSwitch deletes a virtual switch
func (m *Manager) DeleteSwitch(name string) error {
	// First, remove all member interfaces
	switchInfo, err := m.GetSwitch(name)
	if err == nil {
		for _, iface := range switchInfo.Interfaces {
			cmd := exec.Command("ifconfig", name, "deletem", iface)
			cmd.Run()
		}
	}

	// Destroy the bridge
	cmd := exec.Command("ifconfig", name, "destroy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to destroy bridge: %w", err)
	}

	return nil
}

// GetSwitch gets information about a switch
func (m *Manager) GetSwitch(name string) (*SwitchInfo, error) {
	cmd := exec.Command("ifconfig", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge info: %w", err)
	}

	info := &SwitchInfo{
		Name:       name,
		Type:       "bridge",
		Interfaces: []string{},
	}

	// Parse output to find member interfaces
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "member:") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				info.Interfaces = append(info.Interfaces, parts[1])
			}
		}
	}

	return info, nil
}

// ListSwitches lists all virtual switches
func (m *Manager) ListSwitches() ([]*SwitchInfo, error) {
	var switches []*SwitchInfo

	// List all bridge interfaces
	cmd := exec.Command("ifconfig", "-a", "-g", "bridge")
	output, err := cmd.Output()
	if err != nil {
		// No bridges found
		return switches, nil
	}

	names := strings.Fields(string(output))
	for _, name := range names {
		info, err := m.GetSwitch(name)
		if err == nil {
			switches = append(switches, info)
		}
	}

	return switches, nil
}

// AttachToBridge attaches a TAP interface to a bridge
func (m *Manager) AttachToBridge(bridgeName, tapInterface string) error {
	cmd := exec.Command("ifconfig", bridgeName, "addm", tapInterface)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to attach to bridge: %w", err)
	}

	return nil
}

// DetachFromBridge detaches a TAP interface from a bridge
func (m *Manager) DetachFromBridge(bridgeName, tapInterface string) error {
	cmd := exec.Command("ifconfig", bridgeName, "deletem", tapInterface)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to detach from bridge: %w", err)
	}

	return nil
}

// CreateTAP creates a TAP interface
func (m *Manager) CreateTAP() (string, error) {
	cmd := exec.Command("ifconfig", "tap", "create")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create TAP: %w", err)
	}

	// Parse output to get interface name
	name := strings.TrimSpace(string(output))
	return name, nil
}

// DestroyTAP destroys a TAP interface
func (m *Manager) DestroyTAP(name string) error {
	cmd := exec.Command("ifconfig", name, "destroy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to destroy TAP: %w", err)
	}

	return nil
}

// GetDefaultBridge returns the default bridge name
func (m *Manager) GetDefaultBridge() string {
	return m.defaultBridge
}
