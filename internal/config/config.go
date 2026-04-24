package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the global server configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Paths    PathsConfig    `yaml:"paths"`
	Defaults DefaultsConfig `yaml:"defaults"`
	VNC      VNCConfig      `yaml:"vnc"`
	Limits   LimitsConfig   `yaml:"limits"`
	Console  ConsoleConfig  `yaml:"console"`
	Cleanup  CleanupConfig  `yaml:"cleanup"`
}

// ServerConfig holds server-specific settings
type ServerConfig struct {
	Transport string `yaml:"transport"`
	Port      int    `yaml:"port"`
	Bind      string `yaml:"bind"`
	LogLevel  string `yaml:"log_level"`
}

// PathsConfig holds directory paths
type PathsConfig struct {
	VMConfigDir string `yaml:"vm_config_dir"`
	StateDir    string `yaml:"state_dir"`
	ISODir      string `yaml:"iso_dir"`
	DiskDir     string `yaml:"disk_dir"`
	TemplateDir string `yaml:"template_dir"`
	CloudInitDir  string `yaml:"cloud_init_dir"`
	LogDir      string `yaml:"log_dir"`
	ConsoleLogDir string `yaml:"console_log_dir"`
}

// DefaultsConfig holds default VM settings
type DefaultsConfig struct {
	CPU          int    `yaml:"cpu"`
	Memory       string `yaml:"memory"`
	DiskSize     string `yaml:"disk_size"`
	DiskType     string `yaml:"disk_type"`
	Zpool        string `yaml:"zpool"`
	NetworkBridge string `yaml:"network_bridge"`
	Loader       string `yaml:"loader"`
	UEFIFirmware string `yaml:"uefi_firmware"`
}

// VNCConfig holds VNC framebuffer settings
type VNCConfig struct {
	Enabled bool   `yaml:"enabled"`
	BasePort int    `yaml:"base_port"`
	Bind    string `yaml:"bind"`
	Width   int    `yaml:"width"`
	Height  int    `yaml:"height"`
}

// LimitsConfig holds resource limits
type LimitsConfig struct {
	MaxVMs             int    `yaml:"max_vms"`
	MaxCPUPerVM        int    `yaml:"max_cpu_per_vm"`
	MaxMemoryPerVM     string `yaml:"max_memory_per_vm"`
	MaxISOStorage      string `yaml:"max_iso_storage"`
	MaxDiskStorage     string `yaml:"max_disk_storage"`
	MaxTemplateStorage string `yaml:"max_template_storage"`
	MaxISOSize         string `yaml:"max_iso_size"`
	MaxDiskSize        string `yaml:"max_disk_size"`
}

// ConsoleConfig holds console settings
type ConsoleConfig struct {
	Persist      bool   `yaml:"persist"`
	LogDir       string `yaml:"log_dir"`
	MaxLogSize   string `yaml:"max_log_size"`
	MaxLogFiles  int    `yaml:"max_log_files"`
}

// CleanupConfig holds cleanup settings
type CleanupConfig struct {
	ISOMaxAge       string `yaml:"iso_max_age"`
	TemplateMaxAge  string `yaml:"template_max_age"`
	DryRun          bool   `yaml:"dry_run"`
}

// VMConfig represents a single VM configuration
type VMConfig struct {
	Name    string       `yaml:"name"`
	CPU     int          `yaml:"cpu"`
	Memory  string       `yaml:"memory"`
	Boot    BootConfig   `yaml:"boot"`
	Disks   []DiskConfig `yaml:"disks"`
	Network []NetConfig  `yaml:"network"`
	Console []ConsoleDeviceConfig `yaml:"console"`
	VNC     VNCDeviceConfig `yaml:"vnc,omitempty"`
	Flags   []string     `yaml:"flags,omitempty"`
}

// BootConfig holds boot loader settings
type BootConfig struct {
	Loader   string `yaml:"loader"`
	Firmware string `yaml:"firmware,omitempty"`
	Disk     string `yaml:"disk,omitempty"`
	Device   string `yaml:"device,omitempty"`
}

// DiskConfig holds disk settings
type DiskConfig struct {
	Type     string `yaml:"type"`
	Path     string `yaml:"path"`
	Size     string `yaml:"size,omitempty"`
	ReadOnly bool   `yaml:"readonly,omitempty"`
}

// NetConfig holds network settings
type NetConfig struct {
	Type   string `yaml:"type"`
	Bridge string `yaml:"bridge"`
	MAC    string `yaml:"mac"`
}

// ConsoleDeviceConfig holds console device settings
type ConsoleDeviceConfig struct {
	Type   string `yaml:"type"`
	Device string `yaml:"device"`
}

// VNCDeviceConfig holds VNC device settings
type VNCDeviceConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Width   int    `yaml:"width"`
	Height  int    `yaml:"height"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults sets default values for missing configuration
func (c *Config) setDefaults() {
	if c.Server.Transport == "" {
		c.Server.Transport = "stdio"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Bind == "" {
		c.Server.Bind = "127.0.0.1"
	}
	if c.Server.LogLevel == "" {
		c.Server.LogLevel = "info"
	}

	if c.Paths.VMConfigDir == "" {
		c.Paths.VMConfigDir = "/usr/local/etc/bhyve-mcp/vms"
	}
	if c.Paths.StateDir == "" {
		c.Paths.StateDir = "/var/db/bhyve-mcp"
	}
	if c.Paths.ISODir == "" {
		c.Paths.ISODir = "/var/lib/bhyve-mcp/isos"
	}
	if c.Paths.DiskDir == "" {
		c.Paths.DiskDir = "/var/lib/bhyve-mcp/disks"
	}
	if c.Paths.TemplateDir == "" {
		c.Paths.TemplateDir = "/var/lib/bhyve-mcp/templates"
	}
	if c.Paths.CloudInitDir == "" {
		c.Paths.CloudInitDir = "/var/lib/bhyve-mcp/cloud-init"
	}
	if c.Paths.LogDir == "" {
		c.Paths.LogDir = "/var/log/bhyve-mcp"
	}
	if c.Paths.ConsoleLogDir == "" {
		c.Paths.ConsoleLogDir = "/var/log/bhyve-mcp/console"
	}

	if c.Defaults.CPU == 0 {
		c.Defaults.CPU = 2
	}
	if c.Defaults.Memory == "" {
		c.Defaults.Memory = "2048M"
	}
	if c.Defaults.DiskSize == "" {
		c.Defaults.DiskSize = "20G"
	}
	if c.Defaults.DiskType == "" {
		c.Defaults.DiskType = "zvol"
	}
	if c.Defaults.Zpool == "" {
		c.Defaults.Zpool = "zroot"
	}
	if c.Defaults.NetworkBridge == "" {
		c.Defaults.NetworkBridge = "vmbridge0"
	}
	if c.Defaults.Loader == "" {
		c.Defaults.Loader = "uefi"
	}
	if c.Defaults.UEFIFirmware == "" {
		c.Defaults.UEFIFirmware = "/usr/local/share/uefi-firmware/BHYVE_UEFI.fd"
	}

	if c.VNC.BasePort == 0 {
		c.VNC.BasePort = 5900
	}
	if c.VNC.Bind == "" {
		c.VNC.Bind = "127.0.0.1"
	}
	if c.VNC.Width == 0 {
		c.VNC.Width = 1024
	}
	if c.VNC.Height == 0 {
		c.VNC.Height = 768
	}

	if c.Limits.MaxVMs == 0 {
		c.Limits.MaxVMs = 10
	}
	if c.Limits.MaxCPUPerVM == 0 {
		c.Limits.MaxCPUPerVM = 8
	}
	if c.Limits.MaxMemoryPerVM == "" {
		c.Limits.MaxMemoryPerVM = "32768M"
	}
	if c.Limits.MaxISOStorage == "" {
		c.Limits.MaxISOStorage = "50G"
	}
	if c.Limits.MaxDiskStorage == "" {
		c.Limits.MaxDiskStorage = "500G"
	}
	if c.Limits.MaxTemplateStorage == "" {
		c.Limits.MaxTemplateStorage = "200G"
	}
	if c.Limits.MaxISOSize == "" {
		c.Limits.MaxISOSize = "10G"
	}
	if c.Limits.MaxDiskSize == "" {
		c.Limits.MaxDiskSize = "100G"
	}

	if c.Console.MaxLogFiles == 0 {
		c.Console.MaxLogFiles = 10
	}
}

// LoadVMConfig loads a VM configuration from a YAML file
func LoadVMConfig(path string) (*VMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read VM config file: %w", err)
	}

	var vmCfg VMConfig
	if err := yaml.Unmarshal(data, &vmCfg); err != nil {
		return nil, fmt.Errorf("failed to parse VM config: %w", err)
	}

	return &vmCfg, nil
}

// SaveVMConfig saves a VM configuration to a YAML file
func SaveVMConfig(path string, cfg *VMConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal VM config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write VM config: %w", err)
	}

	return nil
}

// DefaultConfig returns a configuration with all default values
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.setDefaults()
	return cfg
}
