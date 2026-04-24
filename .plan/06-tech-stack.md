# Tech Stack and Implementation

## Language Decision

Per CloudBSD guidelines, we evaluate:

| Language | Pros | Cons | Verdict |
|----------|------|------|---------|
| **C** | Native FreeBSD, direct `libvmmapi`, fastest | Verbose, manual memory management, slow dev | ❌ Too slow for MCP prototyping |
| **C++** | Native, `libvmmapi`, modern abstractions | Complex build, slower iteration | ❌ Overkill for this scope |
| **Rust** | Memory safety, performance, good FFI | Steep learning curve, smaller FreeBSD community | ⚠️ Good but slower dev |
| **Go** | Static binary, fast build, good concurrency, easy MCP SDK | Slightly larger binary, CGO for `libvmmapi` | ✅ **Recommended** |
| **Python** | Fastest dev, excellent MCP SDK (`mcp` package), huge ecosystem | Slower runtime, dependency management | ✅ **Also viable** |

### Decision: Go (Primary) with Python fallback

**Primary: Go**
- Static binary = easy deployment on FreeBSD.
- Fast compilation.
- Good standard library for subprocess management (wrapping `bhyve`, `bhyvectl`).
- Can use `cgo` to link `libvmmapi` if needed for advanced features.
- Existing Go MCP SDKs available.

**Fallback: Python**
- If Go MCP SDK proves immature, Python's official `mcp` package is very stable.
- Easier for rapid prototyping.

## MCP SDK

### Go

- `github.com/mark3labs/mcp-go` — popular Go MCP SDK.
- Implements stdio and SSE transports.
- Supports tools, resources, and prompts.

### Python

- `mcp` package from `pip install mcp`.
- Official SDK from Anthropic.
- Very mature and well-documented.

## Key Dependencies

### Runtime

| Dependency | Purpose | FreeBSD Package |
|------------|---------|-----------------|
| `bhyve` | VM hypervisor | base system |
| `bhyvectl` | VM control | base system |
| `nmdm` | Serial consoles | base system |
| `zfs` | Storage backend | base system |
| `if_bridge` / `if_tap` | Networking | base system |
| `libvmmapi` | Advanced VM stats | base system (devel) |

### Build / Optional

| Dependency | Purpose | FreeBSD Package |
|------------|---------|-----------------|
| `libvncclient` | VNC screenshot capture | `libvncserver` |
| `qemu-img` | QCOW2 disk support | `qemu-tools` |
| `grub2-bhyve` | GRUB bootloader | `grub2-bhyve` |
| `uefi-edk2-bhyve` | UEFI firmware | `uefi-edk2-bhyve` |

## Project Structure (Go)

```
bhyve-mcp/
├── cmd/
│   └── bhyve-mcp/
│       └── main.go
├── internal/
│   ├── config/       # Config parsing
│   ├── mcp/          # MCP server setup
│   ├── vm/           # VM lifecycle management
│   ├── disk/         # Disk image management
│   ├── net/          # Network / bridge / tap
│   ├── console/      # Serial / VNC console
│   ├── host/         # Host info, resources
│   └── store/        # State persistence
├── pkg/
│   └── bhyve/        # bhyve CLI wrapper
├── configs/
│   ├── bhyve_mcp      # rc.d script
│   └── config.yaml    # example config
├── docs/
│   └── ...
├── go.mod
├── Makefile
└── README.md
```

## Build and Packaging

### Makefile Targets

```makefile
.PHONY: build install clean package

build:
	go build -o bin/bhyve-mcp ./cmd/bhyve-mcp

install:
	install -m 755 bin/bhyve-mcp /usr/local/bin/
	install -m 644 configs/bhyve_mcp /usr/local/etc/rc.d/
	mkdir -p /usr/local/etc/bhyve-mcp/vms
	mkdir -p /var/db/bhyve-mcp
	mkdir -p /var/lib/bhyve-mcp/isos

clean:
	rm -rf bin/

package: build
	# Create FreeBSD pkg or tarball
```

### FreeBSD Port / Package

Long-term goal: create a FreeBSD port in `sysutils/bhyve-mcp`.

## Configuration Schema

```yaml
server:
  transport: stdio        # or sse
  port: 8080              # for sse
  bind: 127.0.0.1
  log_level: info

paths:
  vm_config_dir: /usr/local/etc/bhyve-mcp/vms
  state_dir: /var/db/bhyve-mcp
  iso_dir: /var/lib/bhyve-mcp/isos
  disk_dir: /var/lib/bhyve-mcp/disks

defaults:
  cpu: 2
  memory: 2048M
  disk_size: 20G
  disk_type: zvol
  zpool: zroot
  network_bridge: vmbridge0
  loader: uefi
  uefi_firmware: /usr/local/share/uefi-firmware/BHYVE_UEFI.fd

vnc:
  enabled: true
  base_port: 5900
  bind: 127.0.0.1
  width: 1024
  height: 768

limits:
  max_vms: 10
  max_cpu_per_vm: 8
  max_memory_per_vm: 32768M
```
