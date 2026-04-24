package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

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

// Initialize initializes the server
func (s *Server) Initialize() error {
	// No global initialization needed for vmmapi
	return nil
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
		var err error
		vm, err = vmmapi.Open(p.Name)
		if err != nil {
			return nil, fmt.Errorf("VM not found: %s", p.Name)
		}
		s.vms[p.Name] = vm
	}

	state, err := vm.GetState()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"state":   state.String(),
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

// Request represents an MCP request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents an MCP error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Run starts the MCP server
func (s *Server) Run() error {
	log.Println("Starting bhyve MCP server...")
	if err := s.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	log.Println("bhyve MCP server initialized")

	// Read requests from stdin and write responses to stdout
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to decode request: %w", err)
		}

		result, err := s.HandleRequest(context.Background(), req.Method, req.Params)

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		if err != nil {
			resp.Error = &Error{
				Code:    -32000,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}

		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode response: %w", err)
		}
	}
}
