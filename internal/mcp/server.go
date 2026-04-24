package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mlapointe/bhyve-mcp/internal/config"
	"github.com/mlapointe/bhyve-mcp/internal/vm"
	"github.com/mlapointe/bhyve-mcp/internal/disk"
	"github.com/mlapointe/bhyve-mcp/internal/net"
	"github.com/mlapointe/bhyve-mcp/internal/store"
)

// Server represents the MCP server for bhyve VM management
type Server struct {
	vmManager    *vm.Manager
	diskManager  *disk.Manager
	netManager   *net.Manager
	isoStore     *store.ISOStore
	templateStore *store.TemplateStore
	stateStore   *store.Store
	config       *config.Config
}

// NewServer creates a new MCP server instance
func NewServer() *Server {
	return &Server{}
}

// Initialize initializes the server
func (s *Server) Initialize() error {
	// Load or create default configuration
	var err error
	s.config = config.DefaultConfig()

	// Initialize state store
	s.stateStore, err = store.NewStore(s.config.Paths.StateDir)
	if err != nil {
		return fmt.Errorf("failed to initialize state store: %w", err)
	}

	// Initialize VM manager
	s.vmManager = vm.NewManager(s.config)
	if err := s.vmManager.LoadAllVMs(); err != nil {
		log.Printf("Warning: failed to load some VMs: %v", err)
	}

	// Initialize disk manager
	s.diskManager = disk.NewManager(s.config.Paths.DiskDir, s.config.Defaults.Zpool)

	// Initialize network manager
	s.netManager = net.NewManager(s.config.Defaults.NetworkBridge)

	// Initialize ISO store
	s.isoStore, err = store.NewISOStore(s.stateStore, s.config.Paths.ISODir)
	if err != nil {
		return fmt.Errorf("failed to initialize ISO store: %w", err)
	}

	// Initialize template store
	s.templateStore, err = store.NewTemplateStore(s.stateStore, s.config.Paths.TemplateDir)
	if err != nil {
		return fmt.Errorf("failed to initialize template store: %w", err)
	}

	log.Println("MCP server initialized")
	return nil
}

// HandleRequest processes an MCP request
func (s *Server) HandleRequest(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
	switch method {
	// VM lifecycle
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
	case "vm/status":
		return s.handleGetVMStatus(params)
	
	// Disk management
	case "disk/create":
		return s.handleCreateDisk(params)
	case "disk/delete":
		return s.handleDeleteDisk(params)
	case "disk/resize":
		return s.handleResizeDisk(params)
	case "disk/clone":
		return s.handleCloneDisk(params)
	case "disk/list":
		return s.handleListDisks()
	
	// ISO management
	case "iso/list":
		return s.handleListISOs()
	case "iso/delete":
		return s.handleDeleteISO(params)
	
	// Template management
	case "template/list":
		return s.handleListTemplates()
	
	// Network management
	case "net/switch/list":
		return s.handleListSwitches()
	case "net/switch/create":
		return s.handleCreateSwitch(params)
	case "net/switch/delete":
		return s.handleDeleteSwitch(params)
	
	// Host info
	case "host/info":
		return s.handleHostInfo()
	
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

type CreateVMParams struct {
	Name   string `json:"name"`
	CPU    int    `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

func (s *Server) handleCreateVM(params json.RawMessage) (interface{}, error) {
	var p CreateVMParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := s.vmManager.Create(p.Name, p.CPU, p.Memory); err != nil {
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

	if err := s.vmManager.Start(p.Name); err != nil {
		return nil, err
	}

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

	if err := s.vmManager.Stop(p.Name); err != nil {
		return nil, err
	}

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

	if err := s.vmManager.Destroy(p.Name); err != nil {
		return nil, err
	}

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

	state, err := s.vmManager.GetState(p.Name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"state":   string(state),
	}, nil
}

func (s *Server) handleGetVMStatus(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	status, err := s.vmManager.GetStatus(p.Name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"status":  status,
	}, nil
}

func (s *Server) handleListVMs() (interface{}, error) {
	vms := s.vmManager.List()
	result := make([]map[string]interface{}, 0, len(vms))
	for _, vm := range vms {
		result = append(result, map[string]interface{}{
			"name":  vm.Name,
			"state": string(vm.State),
			"cpu":   vm.Config.CPU,
			"memory": vm.Config.Memory,
		})
	}

	return map[string]interface{}{
		"success": true,
		"vms":     result,
	}, nil
}

// Disk management handlers

type CreateDiskParams struct {
	Name string `json:"name"`
	Size string `json:"size"`
	Type string `json:"type,omitempty"`
}

func (s *Server) handleCreateDisk(params json.RawMessage) (interface{}, error) {
	var p CreateDiskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	diskType := disk.DiskType(p.Type)
	if diskType == "" {
		diskType = disk.DiskTypeRaw
	}

	if err := s.diskManager.Create(p.Name, p.Size, diskType); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"type":    string(diskType),
		"size":    p.Size,
	}, nil
}

func (s *Server) handleDeleteDisk(params json.RawMessage) (interface{}, error) {
	var p CreateDiskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	diskType := disk.DiskType(p.Type)
	if diskType == "" {
		diskType = disk.DiskTypeRaw
	}

	if err := s.diskManager.Delete(p.Name, diskType); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
	}, nil
}

type ResizeDiskParams struct {
	Name string `json:"name"`
	Size string `json:"size"`
	Type string `json:"type,omitempty"`
}

func (s *Server) handleResizeDisk(params json.RawMessage) (interface{}, error) {
	var p ResizeDiskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	diskType := disk.DiskType(p.Type)
	if diskType == "" {
		diskType = disk.DiskTypeRaw
	}

	if err := s.diskManager.Resize(p.Name, p.Size, diskType); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
		"size":    p.Size,
	}, nil
}

type CloneDiskParams struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
	Type   string `json:"type,omitempty"`
}

func (s *Server) handleCloneDisk(params json.RawMessage) (interface{}, error) {
	var p CloneDiskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	diskType := disk.DiskType(p.Type)
	if diskType == "" {
		diskType = disk.DiskTypeRaw
	}

	if err := s.diskManager.Clone(p.Source, p.Dest, diskType); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"source":  p.Source,
		"dest":    p.Dest,
	}, nil
}

func (s *Server) handleListDisks() (interface{}, error) {
	disks, err := s.diskManager.List()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(disks))
	for _, d := range disks {
		result = append(result, map[string]interface{}{
			"name": d.Name,
			"type": string(d.Type),
			"path": d.Path,
			"size": d.Size,
		})
	}

	return map[string]interface{}{
		"success": true,
		"disks":   result,
	}, nil
}

// ISO management handlers

func (s *Server) handleListISOs() (interface{}, error) {
	isos := s.isoStore.List()
	result := make([]map[string]interface{}, 0, len(isos))
	for _, iso := range isos {
		result = append(result, map[string]interface{}{
			"name":       iso.Name,
			"url":        iso.URL,
			"size":       iso.Size,
			"downloaded": iso.Downloaded,
			"verified":   iso.Verified,
		})
	}

	return map[string]interface{}{
		"success": true,
		"isos":    result,
	}, nil
}

func (s *Server) handleDeleteISO(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := s.isoStore.Delete(p.Name); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
	}, nil
}

// Template management handlers

func (s *Server) handleListTemplates() (interface{}, error) {
	templates := s.templateStore.List()
	result := make([]map[string]interface{}, 0, len(templates))
	for _, t := range templates {
		result = append(result, map[string]interface{}{
			"name":      t.Name,
			"source_vm": t.SourceVM,
			"created":   t.Created,
			"size":      t.Size,
			"disk_type": t.DiskType,
		})
	}

	return map[string]interface{}{
		"success":   true,
		"templates": result,
	}, nil
}

// Network management handlers

type CreateSwitchParams struct {
	Name              string `json:"name"`
	PhysicalInterface string `json:"physical_interface,omitempty"`
}

func (s *Server) handleCreateSwitch(params json.RawMessage) (interface{}, error) {
	var p CreateSwitchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := s.netManager.CreateSwitch(p.Name, p.PhysicalInterface); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
	}, nil
}

func (s *Server) handleDeleteSwitch(params json.RawMessage) (interface{}, error) {
	var p VMNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := s.netManager.DeleteSwitch(p.Name); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"name":    p.Name,
	}, nil
}

func (s *Server) handleListSwitches() (interface{}, error) {
	switches, err := s.netManager.ListSwitches()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(switches))
	for _, sw := range switches {
		result = append(result, map[string]interface{}{
			"name":       sw.Name,
			"type":       sw.Type,
			"interfaces": sw.Interfaces,
		})
	}

	return map[string]interface{}{
		"success":  true,
		"switches": result,
	}, nil
}

// Host info handler

func (s *Server) handleHostInfo() (interface{}, error) {
	// Return basic host information
	return map[string]interface{}{
		"success": true,
		"info": map[string]interface{}{
			"platform":        "freebsd",
			"default_bridge":  s.config.Defaults.NetworkBridge,
			"default_zpool":   s.config.Defaults.Zpool,
			"max_vms":         s.config.Limits.MaxVMs,
			"max_cpu_per_vm":  s.config.Limits.MaxCPUPerVM,
			"max_memory_per_vm": s.config.Limits.MaxMemoryPerVM,
		},
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
