package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mlapointe/bhyve-mcp/internal/vmmapi"
)

// Server represents the MCP server for bhyve VM management
type Server struct {
	vms map[string]*vmmapi.VM
}

// NewServer creates a new MCP server instance
func NewServer() *Server {
	return &Server{
		vms: make(map[string]*vmmapi.VM),
	}
}

// Initialize initializes the server and vmmapi
func (s *Server) Initialize() error {
	return vmmapi.Init()
}

// HandleRequest processes an MCP request
func (s *Server) HandleRequest(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
	switch method {
	case "vm/create":
		return s.handleCreateVM(params)
	case "vm/start":
		return s.handleStartVM(params)
	case "vm/stop":
		return s.handleStopVM(params)
	case "vm/destroy":
		return s.handleDestroyVM(params)
	case "vm/state":
		return s.handleGetVMState(params)
	case "vm/list":
		return s.handleListVMs()
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

type CreateVMParams struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateVM(params json.RawMessage) (interface{}, error) {
	var p CreateVMParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if _, err := vmmapi.Create(p.Name); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
	}, nil
}

type VMNameParams struct {
	Name string `json:"name"`
}

func (s *Server) handleStartVM(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	vm, exists := s.vms[p.Name]
	if !exists {
		// Try to open the VM
		var err error
		vm, err = vmmapi.Open(p.Name)
		if err != nil {
			return nil, fmt.Errorf("VM not found: %s", p.Name)
		}
		s.vms[p.Name] = vm
	}

	// For now, just return success - actual start implementation needs vm_run
	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"status":  "started",
	}, nil
}

func (s *Server) handleStopVM(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	vm, exists := s.vms[p.Name]
	if !exists {
		var openErr error
		vm, openErr = vmmapi.Open(p.Name)
		if openErr != nil {
			return nil, fmt.Errorf("VM not found: %s", p.Name)
		}
		s.vms[p.Name] = vm
	}

	// Use vm_suspend to stop the VM
	// For now, just close the connection
	vm.Close()
	delete(s.vms, p.Name)

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"status":  "stopped",
	}, nil
}

func (s *Server) handleDestroyVM(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	vm, exists := s.vms[p.Name]
	if !exists {
		var err error
		vm, err = vmmapi.Open(p.Name)
		if err != nil {
			return nil, fmt.Errorf("VM not found: %s", p.Name)
		}
	}

	vm.Destroy()
	delete(s.vms, p.Name)

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"status":  "destroyed",
	}, nil
}

func (s *Server) handleGetVMState(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	vm, exists := s.vms[p.Name]
	if !exists {
		return nil, fmt.Errorf("VM not found: %s", p.Name)
	}

	state, err := vm.GetState()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"state":   state,
	}, nil
}

func (s *Server) handleListVMs() (interface{}, error) {
	vms := make([]map[string]interface{}, 0, len(s.vms))
	for name, vm := range s.vms {
		state, _ := vm.GetState()
		vms = append(vms, map[string]interface{}{
			"name":  name,
			"state": state,
		})
	}

	return map[string]interface{}{
		"success": true,
		"vms":     vms,
	}, nil
}

// Run starts the MCP server
func (s *Server) Run() error {
	log.Println("Starting bhyve MCP server...")
	if err := s.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	log.Println("bhyve MCP server initialized")

	// Read requests from stdin and write responses to stdout
	decoder := json.NewDecoder(nil)
	encoder := json.NewEncoder(nil)

	for {
		// This will be implemented with proper stdin/stdout handling
		// For now, this is a placeholder
		select {
		case <-context.Background().Done():
			return context.Background().Err()
		}
	}
}
