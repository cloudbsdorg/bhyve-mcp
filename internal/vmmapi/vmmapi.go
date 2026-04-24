package vmmapi

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -lvmmapi
#include <vmmapi.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// VM represents a bhyve virtual machine
type VM struct {
	handle *C.struct_vmctx
	name   string
}

// Create creates a new virtual machine
func Create(name string) (int, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	ret := C.vm_create(cName)
	if ret != 0 {
		return ret, fmt.Errorf("failed to create VM: %s (error: %d)", name, ret)
	}
	return ret, nil
}

// Open opens an existing virtual machine
func Open(name string) (*VM, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.vm_open(cName)
	if handle == nil {
		return nil, fmt.Errorf("failed to open VM: %s", name)
	}

	return &VM{
		handle: handle,
		name:   name,
	}, nil
}

// Close closes a virtual machine connection
func (vm *VM) Close() {
	if vm.handle != nil {
		C.vm_close(vm.handle)
		vm.handle = nil
	}
}

// Destroy destroys a virtual machine
func (vm *VM) Destroy() {
	if vm.handle != nil {
		C.vm_destroy(vm.handle)
		vm.handle = nil
	}
}

// State represents the state of a VM
type State int

const (
	StateUnknown State = iota
	StateStopped
	StateRunning
	StatePaused
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	default:
		return "unknown"
	}
}

// GetState gets the current state of a virtual machine
func (vm *VM) GetState() (State, error) {
	// For now, return stopped as the basic implementation
	// This will be enhanced with proper state detection
	return StateStopped, nil
}

// SetupMemory sets up memory for the VM
func (vm *VM) SetupMemory(size uint64) error {
	ret := C.vm_setup_memory(vm.handle, C.size_t(size), C.VM_MMAP_STYLE_SHARED)
	if ret != 0 {
		return fmt.Errorf("failed to setup memory: %d", ret)
	}
	return nil
}
