package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/mlapointe/bhyve-mcp/internal/config"
	"github.com/mlapointe/bhyve-mcp/internal/vmmapi"
)

// VMState represents the state of a VM
type VMState string

const (
	StateDefined    VMState = "defined"
	StateStarting   VMState = "starting"
	StateRunning    VMState = "running"
	StateStopping   VMState = "stopping"
	StateStopped    VMState = "stopped"
	StateError      VMState = "error"
)

// VM represents a managed bhyve virtual machine
type VM struct {
	Name       string
	Config     *config.VMConfig
	State      VMState
	Process    *os.Process
	vmmapiVM   *vmmapi.VM
	mu         sync.Mutex
}

// Manager manages bhyve VMs
type Manager struct {
	config      *config.Config
	vms         map[string]*VM
	mu          sync.RWMutex
	vncPortNext int
}

// NewManager creates a new VM manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:      cfg,
		vms:         make(map[string]*VM),
		vncPortNext: cfg.VNC.BasePort,
	}
}

// List returns all managed VMs
func (m *Manager) List() []*VM {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vms := make([]*VM, 0, len(m.vms))
	for _, vm := range m.vms {
		vms = append(vms, vm)
	}
	return vms
}

// Get returns a VM by name
func (m *Manager) Get(name string) (*VM, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, exists := m.vms[name]
	if !exists {
		return nil, fmt.Errorf("VM not found: %s", name)
	}
	return vm, nil
}

// Create creates a new VM configuration
func (m *Manager) Create(name string, cpu int, memory string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.vms[name]; exists {
		return fmt.Errorf("VM already exists: %s", name)
	}

	// Use defaults if not specified
	if cpu == 0 {
		cpu = m.config.Defaults.CPU
	}
	if memory == "" {
		memory = m.config.Defaults.Memory
	}

	vmCfg := &config.VMConfig{
		Name:   name,
		CPU:    cpu,
		Memory: memory,
		Boot: config.BootConfig{
			Loader:   m.config.Defaults.Loader,
			Firmware: m.config.Defaults.UEFIFirmware,
		},
		Network: []config.NetConfig{
			{
				Type:   "tap",
				Bridge: m.config.Defaults.NetworkBridge,
			},
		},
	}

	// Save VM configuration
	configPath := fmt.Sprintf("%s/%s.yaml", m.config.Paths.VMConfigDir, name)
	if err := config.SaveVMConfig(configPath, vmCfg); err != nil {
		return fmt.Errorf("failed to save VM config: %w", err)
	}

	// Create VM kernel object using libvmmapi
	if _, err := vmmapi.Create(name); err != nil {
		return fmt.Errorf("failed to create VM kernel object: %w", err)
	}

	vm := &VM{
		Name:   name,
		Config: vmCfg,
		State:  StateDefined,
	}

	m.vms[name] = vm
	return nil
}

// Start starts a VM
func (m *Manager) Start(name string) error {
	vm, err := m.Get(name)
	if err != nil {
		return err
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.State == StateRunning {
		return fmt.Errorf("VM is already running: %s", name)
	}

	vm.State = StateStarting

	// Open VM via libvmmapi
	vmmVM, err := vmmapi.Open(name)
	if err != nil {
		vm.State = StateError
		return fmt.Errorf("failed to open VM: %w", err)
	}
	vm.vmmapiVM = vmmVM

	// Setup memory
	// Parse memory size (simplified - should handle M/G suffixes)
	memSize := uint64(2048 * 1024 * 1024) // default 2GB
	if err := vmmVM.SetupMemory(memSize); err != nil {
		vm.State = StateError
		return fmt.Errorf("failed to setup memory: %w", err)
	}

	// Build bhyve command
	cmd, err := m.buildBhyveCommand(vm)
	if err != nil {
		vm.State = StateError
		return fmt.Errorf("failed to build bhyve command: %w", err)
	}

	// Start bhyve process
	if err := cmd.Start(); err != nil {
		vm.State = StateError
		return fmt.Errorf("failed to start bhyve: %w", err)
	}

	vm.Process = cmd.Process
	vm.State = StateRunning

	return nil
}

// Stop stops a VM gracefully
func (m *Manager) Stop(name string) error {
	vm, err := m.Get(name)
	if err != nil {
		return err
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.State != StateRunning {
		return fmt.Errorf("VM is not running: %s", name)
	}

	vm.State = StateStopping

	// Try to suspend VM via libvmmapi first
	if vm.vmmapiVM != nil {
		// Note: vm_suspend is not yet implemented in vmmapi wrapper
		// For now, just signal the process
	}

	// Send SIGTERM to bhyve process
	if vm.Process != nil {
		if err := vm.Process.Signal(syscall.SIGTERM); err != nil {
			// Force kill if SIGTERM fails
			if err := vm.Process.Kill(); err != nil {
				vm.State = StateError
				return fmt.Errorf("failed to kill VM process: %w", err)
			}
		}

		// Wait for process to exit
		_, err := vm.Process.Wait()
		if err != nil {
			// Process may have already exited
		}
	}

	// Close libvmmapi handle
	if vm.vmmapiVM != nil {
		vm.vmmapiVM.Close()
		vm.vmmapiVM = nil
	}

	vm.State = StateStopped
	return nil
}

// Destroy destroys a VM completely
func (m *Manager) Destroy(name string) error {
	vm, err := m.Get(name)
	if err != nil {
		return err
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Stop if running
	if vm.State == StateRunning {
		if err := m.Stop(name); err != nil {
			return fmt.Errorf("failed to stop VM before destroy: %w", err)
		}
	}

	// Destroy via libvmmapi
	if vm.vmmapiVM != nil {
		vm.vmmapiVM.Destroy()
		vm.vmmapiVM = nil
	} else {
		// Try to open and destroy
		vmmVM, err := vmmapi.Open(name)
		if err == nil {
			vmmVM.Destroy()
			vmmVM.Close()
		}
	}

	// Remove configuration file
	configPath := fmt.Sprintf("%s/%s.yaml", m.config.Paths.VMConfigDir, name)
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove VM config: %w", err)
	}

	// Remove from manager
	m.mu.Lock()
	delete(m.vms, name)
	m.mu.Unlock()

	return nil
}

// GetState returns the current state of a VM
func (m *Manager) GetState(name string) (VMState, error) {
	vm, err := m.Get(name)
	if err != nil {
		return StateError, err
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Check if process is still running
	if vm.State == StateRunning && vm.Process != nil {
		if err := vm.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is not running
			vm.State = StateStopped
			if vm.vmmapiVM != nil {
				vm.vmmapiVM.Close()
				vm.vmmapiVM = nil
			}
		}
	}

	return vm.State, nil
}

// GetStatus returns detailed VM status
func (m *Manager) GetStatus(name string) (map[string]interface{}, error) {
	vm, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	status := map[string]interface{}{
		"name":  vm.Name,
		"state": vm.State,
		"cpu":   vm.Config.CPU,
		"memory": vm.Config.Memory,
	}

	// Get state from libvmmapi if available
	if vm.vmmapiVM != nil {
		state, err := vm.vmmapiVM.GetState()
		if err == nil {
			status["vmmapi_state"] = state.String()
		}
	}

	return status, nil
}

// buildBhyveCommand constructs the bhyve command line
func (m *Manager) buildBhyveCommand(vm *VM) (*exec.Cmd, error) {
	args := []string{}

	// CPU count
	args = append(args, "-c", fmt.Sprintf("%d", vm.Config.CPU))

	// Memory
	args = append(args, "-m", vm.Config.Memory)

	// Common flags
	args = append(args, "-H", "-w", "-u")

	// Slot 0: hostbridge
	args = append(args, "-s", "0,hostbridge")

	// Slot 1: LPC (for bootrom)
	args = append(args, "-s", "1,lpc")

	// Bootrom
	if vm.Config.Boot.Firmware != "" {
		args = append(args, "-l", fmt.Sprintf("bootrom,%s", vm.Config.Boot.Firmware))
	}

	// Slots for disks (starting from slot 2)
	slot := 2
	for _, disk := range vm.Config.Disks {
		var diskArg string
		switch disk.Type {
		case "zvol":
			diskArg = fmt.Sprintf("virtio-blk,%s", disk.Path)
		case "file":
			diskArg = fmt.Sprintf("virtio-blk,%s", disk.Path)
		default:
			diskArg = fmt.Sprintf("virtio-blk,%s", disk.Path)
		}
		args = append(args, "-s", fmt.Sprintf("%d,%s", slot, diskArg))
		slot++
	}

	// Slots for network
	for range vm.Config.Network {
		// For now, use simple tap interface
		// In production, would create tap and attach to bridge
		args = append(args, "-s", fmt.Sprintf("%d,virtio-net,tap0", slot))
		slot++
	}

	// VNC framebuffer (if enabled)
	if vm.Config.VNC.Enabled || m.config.VNC.Enabled {
		port := m.vncPortNext
		m.vncPortNext++
		args = append(args, "-s", fmt.Sprintf("%d,fbuf,tcp=%s:%d,wait", slot, m.config.VNC.Bind, port))
		args = append(args, "-s", fmt.Sprintf("%d,xhci,tablet", slot+1))
		slot += 2
	}

	// Console (nmdm)
	for _, console := range vm.Config.Console {
		if console.Type == "nmdm" {
			args = append(args, "-l", fmt.Sprintf("com1,%s", console.Device))
		}
	}

	// VM name at the end
	args = append(args, vm.Name)

	cmd := exec.Command("bhyve", args...)
	return cmd, nil
}

// LoadVM loads an existing VM configuration
func (m *Manager) LoadVM(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.vms[name]; exists {
		return fmt.Errorf("VM already loaded: %s", name)
	}

	// Load VM configuration
	configPath := fmt.Sprintf("%s/%s.yaml", m.config.Paths.VMConfigDir, name)
	vmCfg, err := config.LoadVMConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load VM config: %w", err)
	}

	vm := &VM{
		Name:   name,
		Config: vmCfg,
		State:  StateDefined,
	}

	m.vms[name] = vm
	return nil
}

// LoadAllVMs loads all VM configurations
func (m *Manager) LoadAllVMs() error {
	// Read VM config directory
	entries, err := os.ReadDir(m.config.Paths.VMConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No VMs yet
		}
		return fmt.Errorf("failed to read VM config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Remove .yaml extension
		if len(name) > 5 && name[len(name)-5:] == ".yaml" {
			name = name[:len(name)-5]
		}
		if err := m.LoadVM(name); err != nil {
			// Log error but continue loading other VMs
			fmt.Printf("Warning: failed to load VM %s: %v\n", name, err)
		}
	}

	return nil
}

// Shutdown gracefully shuts down all running VMs
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, vm := range m.vms {
		if vm.State == StateRunning {
			if err := m.Stop(name); err != nil {
				lastErr = fmt.Errorf("failed to stop VM %s: %w", name, err)
			}
		}
	}

	return lastErr
}
