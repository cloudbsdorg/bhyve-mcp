# bhyve-mcp — Master Implementation Plan

## 1. Executive Summary

This document outlines the implementation of `bhyve-mcp`, a Model Context Protocol (MCP) server for FreeBSD that enables AI coding agents (Junie, OpenCode, Claude, GitHub Copilot, etc.) to create, manage, and monitor bhyve virtual machines directly via the native `libvmmapi` C library.

**Primary Design Decision:** Go with CGO bindings to `libvmmapi` for VM lifecycle control, plus subprocess management for the `bhyve` device model. This provides direct kernel integration without shell-wrapper fragility.

**Target Platform:** FreeBSD 14.x and later.

**Distribution:** FreeBSD Ports Collection (`sysutils/bhyve-mcp`).

---

## 2. Problem Statement

AI coding agents running in Linux-compatible environments cannot reliably manage FreeBSD bhyve VMs. The Linux compatibility layer interferes with `vm-bhyve`, `bhyvectl`, and native `bhyve` execution. A pure FreeBSD-native MCP server is required to bridge this gap.

### 2.1 Why Not Shell Commands?

Shelling out to `bhyve` and `bhyvectl` is:
- **Fragile**: Parsing stderr and exit codes is error-prone.
- **Slow**: Process spawn overhead for every operation.
- **Limited**: No direct access to VM statistics, memory maps, or vCPU state.
- **Opaque**: Difficult to get structured data back.

### 2.2 Why libvmmapi?

`libvmmapi` is the official FreeBSD C library for bhyve VM management. It provides:
- Direct `ioctl` access to `/dev/vmm`.
- Fine-grained VM lifecycle control (`vm_create`, `vm_destroy`, `vm_run`).
- vCPU register access and interrupt injection.
- Per-VM and per-vCPU statistics.
- Memory segment mapping and device memory creation.

---

## 3. Goals and Non-Goals

### 3.1 Goals

1. **Pure FreeBSD**: Runs natively without Linux compatibility.
2. **MCP Compliant**: Implements Model Context Protocol for tool discovery.
3. **Direct Kernel Integration**: Uses `libvmmapi` for VM lifecycle.
4. **Multi-Agent Support**: Works with Junie, OpenCode, Claude, and any MCP client.
5. **VM Lifecycle**: Create, configure, start, stop, destroy VMs.
6. **Screen Capture**: VNC framebuffer screenshots for visual feedback.
7. **Console I/O**: Serial console read/write for text-mode interaction.
8. **Storage**: ZFS zvol and file-based disk management.
9. **Networking**: Virtual switch and bridge management.
10. **Service Integration**: FreeBSD rc.d service with proper logging.
11. **Ports Quality**: Submittable to FreeBSD Ports Collection.

### 3.2 Non-Goals

- Cross-platform support (Linux, macOS, Windows).
- GUI or web interface.
- Live migration (bhyve does not support this).
- PCI passthrough (out of scope for initial version).
- QEMU/KVM compatibility.

---

## 4. Architecture

### 4.1 High-Level Diagram

```
+-------------+     MCP (stdio/SSE)     +-------------+     CGO/ioctl     +-------------+
|   Junie     |  <------------------->  |  bhyve-mcp  |  <------------->  |  libvmmapi  |
|  OpenCode   |      JSON-RPC tools     |    (Go)     |     bindings      |     (C)       |
|   Claude    |                         |             |                   |             |
+-------------+                         +-------------+                   +-------------+
                                               |                              |
                                               | fork/exec                    | ioctl
                                               v                              v
                                        +-------------+                   +-------------+
                                        |   bhyve     |                   |   /dev/vmm  |
                                        |  (device    |                   |   (kernel)  |
                                        |   model)    |                   |             |
                                        +-------------+                   +-------------+
                                               |
                                               | VNC / nmdm
                                               v
                                        +-------------+
                                        |   Console   |
                                        |  / Screen   |
                                        +-------------+
```

### 4.2 Component Breakdown

| Component | Technology | Responsibility |
|-----------|-----------|--------------|
| MCP Server | Go + `mcp-go` SDK | Transport, tool routing, JSON-RPC |
| VM Manager | Go + CGO | `libvmmapi` bindings, VM lifecycle |
| Device Model | `bhyve` binary | virtio-blk, virtio-net, fbuf, LPC |
| Storage | ZFS CLI / `truncate` | zvol creation, disk images |
| Network | `ifconfig` | tap creation, bridge management |
| Console | `nmdm` + VNC | Serial and graphical console |
| State | SQLite / JSON | VM configuration and runtime state |

---

## 5. MCP Server Design

### 5.1 Transport

- **Primary**: `stdio` — Junie and most agents spawn the server as a subprocess.
- **Secondary**: `SSE` over HTTP — for remote or daemonized usage.

### 5.2 Server Capabilities

| Capability | Description |
|------------|-------------|
| `tools` | Expose VM management tools |
| `logging` | Emit structured logs to client |

### 5.3 Tool Inventory

#### VM Lifecycle

| Tool | Description | libvmmapi or bhyve? |
|------|-------------|---------------------|
| `vm_list` | List all VMs and statuses | `libvmmapi` (`/dev/vmm`) |
| `vm_create` | Create VM configuration and kernel object | `libvmmapi` (`vm_create`) |
| `vm_start` | Start VM (fork bhyve device model) | Hybrid |
| `vm_stop` | Gracefully stop VM | `libvmmapi` (`vm_suspend`) |
| `vm_force_stop` | Forcefully destroy VM | `libvmmapi` (`vm_destroy`) |
| `vm_destroy` | Delete VM and all resources | `libvmmapi` (`vm_destroy`) |
| `vm_status` | Detailed VM status | `libvmmapi` (`vm_get_stats`) |
| `vm_stats` | CPU/memory/disk stats | `libvmmapi` (`vm_get_stats`) |

#### Console and Screen

| Tool | Description | Mechanism |
|------|-------------|-----------|
| `vm_screenshot` | Capture VM screen as base64 PNG | VNC framebuffer (`fbuf`) |
| `vm_send_keys` | Send keystrokes to VM | VNC key events |
| `vm_send_text` | Send text to serial console | `nmdm` write |
| `vm_console_read` | Read recent serial console output | `nmdm` read + ring buffer |

#### Storage

| Tool | Description | Mechanism |
|------|-------------|-----------|
| `disk_create` | Create disk (zvol, file, qcow2) | `zfs create -V` / `truncate` |
| `disk_resize` | Resize disk | `zfs set volsize` / `truncate` |
| `disk_delete` | Delete disk | `zfs destroy` / `rm` |
| `disk_list` | List VM disks | Config + `zfs list` |

#### Networking

| Tool | Description | Mechanism |
|------|-------------|-----------|
| `net_switch_list` | List virtual switches | `ifconfig bridge` |
| `net_switch_create` | Create virtual switch | `ifconfig bridge create` |
| `net_switch_delete` | Delete virtual switch | `ifconfig bridge destroy` |
| `net_bridge_attach` | Attach VM NIC to bridge | `ifconfig bridge addm` |

#### Host

| Tool | Description | Mechanism |
|------|-------------|-----------|
| `host_info` | Host CPU, memory, bhyve support | `sysctl`, `kenv` |

### 5.4 Error Codes

| Code | Meaning |
|------|---------|
| `vm_not_found` | VM does not exist |
| `vm_already_running` | VM is already active |
| `vm_not_running` | VM is not active |
| `disk_not_found` | Disk does not exist |
| `insufficient_resources` | Host lacks CPU/memory |
| `permission_denied` | User cannot perform operation |
| `vmmapi_error` | Wrapped `libvmmapi` errno |

---

## 6. libvmmapi Integration

### 6.1 API Surface

The following `libvmmapi` functions will be bound via CGO:

#### VM Lifecycle
- `vm_create(name)` — Create VM kernel object.
- `vm_open(name)` / `vm_openf(name, flags)` — Open existing VM.
- `vm_close(ctx)` — Close handle.
- `vm_destroy(ctx)` — Destroy VM completely.
- `vm_setup_memory(ctx, len, style)` — Allocate guest RAM.
- `vm_run(vcpu, vmrun)` — Run vCPU (used by bhyve binary, not directly by us).
- `vm_suspend(ctx, how)` — Suspend VM.
- `vm_reinit(ctx)` — Reinitialize VM state.

#### vCPU Control
- `vm_vcpu_open(ctx, vcpuid)` — Open vCPU handle.
- `vm_set_register(vcpu, reg, val)` / `vm_get_register(vcpu, reg, retval)` — GPR access.
- `vcpu_reset(vcpu)` — Reset vCPU.
- `vm_activate_cpu(vcpu)` / `vm_suspend_cpu(vcpu)` / `vm_resume_cpu(vcpu)` — vCPU scheduling.

#### Statistics
- `vm_get_stats(vcpu, tv, entries)` — Per-vCPU stats array.
- `vm_get_stat_desc(ctx, index)` — Stat name lookup.
- `vm_active_cpus(ctx, cpus)` — Active CPU bitmap.
- `vm_suspended_cpus(ctx, cpus)` — Suspended CPU bitmap.

#### Interrupts
- `vm_inject_exception(vcpu, vector, ...)` — Inject exception.
- `vm_lapic_irq(vcpu, vector)` — LAPIC interrupt.
- `vm_ioapic_assert_irq(ctx, irq)` — IOAPIC line assert.
- `vm_inject_nmi(vcpu)` — NMI injection.

### 6.2 CGO Strategy

**Approach:** Inline CGO in Go files.

```go
/*
#include <vmmapi.h>
#include <machine/vmm.h>
#include <errno.h>
*/
import "C"
```

Each `libvmmapi` function gets a thin Go wrapper that:
1. Converts Go strings to C strings (`C.CString`).
2. Calls the C function.
3. Maps `int` return to Go `error`.
4. Frees C allocations.

### 6.3 Thread Safety

`libvmmapi` is **not MT-safe** for statistics (`vm_get_stats`). We will:
- Serialize per-VM operations with a mutex.
- Use a dedicated goroutine per VM for the `bhyve` device model process.
- Protect `vm_get_stats` calls with a global or per-VM mutex.

### 6.4 What Still Uses Subprocesses

| Operation | Why |
|-----------|-----|
| TAP create/destroy | Network stack has no stable API |
| Bridge management | Same |
| ZFS zvol create/destroy | ZFS CLI is the canonical API |
| Disk image creation | No direct API for qcow2/raw |
| `bhyve` device model | Device emulation is userspace-only |

---

## 7. VM Configuration

### 7.1 Configuration File Format

Each VM is defined by a YAML file in `/usr/local/etc/bhyve-mcp/vms/<name>.yaml`.

```yaml
name: ubuntu-vm
cpu: 2
memory: 4096M
boot:
  loader: uefi
  firmware: /usr/local/share/uefi-firmware/BHYVE_UEFI.fd
disks:
  - type: zvol
    path: zroot/vm/ubuntu-vm/disk0
    size: 20G
  - type: file
    path: /var/lib/bhyve-mcp/isos/ubuntu.iso
    readonly: true
network:
  - type: tap
    bridge: vmbridge0
    mac: 00:a0:98:de:ad:01
console:
  - type: nmdm
    device: /dev/nmdm-ubuntu-vm-A
vnc:
  enabled: true
  width: 1024
  height: 768
  port: 5901
flags:
  - -H
  - -w
  - -u
```

### 7.2 Global Configuration

`/usr/local/etc/bhyve-mcp/config.yaml`:

```yaml
server:
  transport: stdio
  port: 8080
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

---

## 8. Console and Screen Capture

### 8.1 VNC Framebuffer (`fbuf`)

bhyve supports a VNC-compatible framebuffer:

```sh
bhyve -s 29,fbuf,tcp=127.0.0.1:5901,w=1024,h=768,wait \
      -s 30,xhci,tablet \
      ...
```

The MCP server connects to `localhost:VM_PORT` via a VNC client library, captures the framebuffer, and encodes it as base64 PNG for the `vm_screenshot` tool.

### 8.2 Serial Console (`nmdm`)

For text-mode VMs:

```sh
bhyve -l com1,/dev/nmdm-ubuntu-vm-A ...
```

The server maintains a ring buffer of serial output and exposes:
- `vm_console_read(vm_name, lines=100)` — Read recent output.
- `vm_send_text(vm_name, text)` — Write to serial console.

### 8.3 Input Injection

| Method | Use Case | Implementation |
|--------|----------|----------------|
| VNC key events | GUI installers, desktop OSes | X11 keysym mapping |
| Serial write | Text-mode, headless servers | Raw bytes to `nmdm` |

---

## 9. FreeBSD Service Integration

### 9.1 rc.d Script

`/usr/local/etc/rc.d/bhyve_mcp`:

```sh
#!/bin/sh
# PROVIDE: bhyve_mcp
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="bhyve_mcp"
rcvar="bhyve_mcp_enable"

load_rc_config $name

: ${bhyve_mcp_enable:="NO"}
: ${bhyve_mcp_user:="bhyve-mcp"}
: ${bhyve_mcp_group:="bhyve-mcp"}
: ${bhyve_mcp_config:="/usr/local/etc/bhyve-mcp/config.yaml"}

pidfile="/var/run/${name}.pid"
command="/usr/local/bin/bhyve-mcp"
command_args="--config ${bhyve_mcp_config}"

run_rc_command "$1"
```

### 9.2 rc.conf Variables

```sh
bhyve_mcp_enable="YES"
bhyve_mcp_user="bhyve-mcp"
bhyve_mcp_group="bhyve-mcp"
bhyve_mcp_config="/usr/local/etc/bhyve-mcp/config.yaml"
bhyve_mcp_transport="stdio"
bhyve_mcp_port="8080"
```

### 9.3 Kernel Modules

Ensure loaded at service start:
- `vmm` — bhyve kernel module.
- `nmdm` — null-modem for serial consoles.
- `if_bridge` — networking bridges.
- `if_tap` — tap interfaces.

### 9.4 Logging

- Facility: `local0` via `syslog`.
- Log file: `/var/log/bhyve-mcp.log`.
- Rotation: `newsyslog`.

---

## 10. FreeBSD Ports Packaging

### 10.1 Port Structure

```
/usr/ports/sysutils/bhyve-mcp/
├── Makefile
├── distinfo
├── pkg-descr
├── pkg-plist
└── files/
    └── bhyve_mcp.in
```

### 10.2 Port Makefile

```makefile
PORTNAME=       bhyve-mcp
DISTVERSION=    0.1.0
CATEGORIES=     sysutils

MAINTAINER=     mlapointe@example.com
COMMENT=        MCP server for managing bhyve virtual machines
WWW=            https://github.com/cloudbsdorg/bhyve-mcp

LICENSE=        BSD2CLAUSE

USES=           go:modules
USE_GITHUB=     yes
GH_ACCOUNT=     cloudbsdorg

GO_MODULE=      github.com/cloudbsdorg/bhyve-mcp

PLIST_FILES=    bin/bhyve-mcp \
                etc/rc.d/bhyve_mcp

SUB_LIST=       BHYVE_MCP_USER=${BHYVE_MCP_USER} \
                BHYVE_MCP_GROUP=${BHYVE_MCP_GROUP}

BHYVE_MCP_USER?=        bhyve-mcp
BHYVE_MCP_GROUP?=       bhyve-mcp

USERS=          ${BHYVE_MCP_USER}
GROUPS=         ${BHYVE_MCP_GROUP}

post-install:
        ${MKDIR} ${STAGEDIR}${PREFIX}/etc/bhyve-mcp/vms
        ${MKDIR} ${STAGEDIR}/var/db/bhyve-mcp
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/isos
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/disks
        ${INSTALL_SCRIPT} ${WRKSRC}/configs/bhyve_mcp ${STAGEDIR}${PREFIX}/etc/rc.d/bhyve_mcp

.include <bsd.port.mk>
```

### 10.3 Port Submission Checklist

- [ ] Builds with `poudriere`.
- [ ] `portlint` passes.
- [ ] `make check-plist` passes.
- [ ] `make stage-qa` passes.
- [ ] Man pages included.
- [ ] rc.d script follows FreeBSD style.
- [ ] DESCR under 80 chars per line.
- [ ] LICENSE file in distfile.

---

## 11. Man Pages

The following man pages will be created:

| Man Page | Section | Description |
|----------|---------|-------------|
| `bhyve-mcp.8` | 8 | Daemon usage, flags, configuration |
| `bhyve-mcp.conf.5` | 5 | Configuration file format |
| `bhyve-mcp-vm.5` | 5 | VM configuration file format |
| `bhyve-mcp-tools.7` | 7 | MCP tool reference for AI agents |

### 11.1 bhyve-mcp.8

```
BHYVE-MCP(8)            FreeBSD System Manager's Manual           BHYVE-MCP(8)

NAME
     bhyve-mcp -- Model Context Protocol server for bhyve VM management

SYNOPSIS
     bhyve-mcp [--config path] [--transport stdio|sse] [--port port]
               [--bind address] [--log-level debug|info|warn|error]

DESCRIPTION
     bhyve-mcp is an MCP server that exposes bhyve virtual machine management
     capabilities to AI coding agents via the Model Context Protocol. It uses
     libvmmapi for direct kernel integration and forks bhyve(8) for device
     emulation.

     The server supports two transports:
     - stdio: For local agent integration (default).
     - sse:  For remote or daemonized usage.

OPTIONS
     --config path      Path to configuration file.
     --transport type   MCP transport: stdio or sse.
     --port port        TCP port for SSE transport.
     --bind address     Bind address for SSE transport.
     --log-level level  Logging verbosity.

FILES
     /usr/local/etc/bhyve-mcp/config.yaml    Global configuration.
     /usr/local/etc/bhyve-mcp/vms/*.yaml     Per-VM configurations.
     /var/db/bhyve-mcp/state.db              Runtime state database.
     /var/log/bhyve-mcp.log                  Log file.

SEE ALSO
     bhyve(8), bhyvectl(8), vmm(4), rc(8), mcp(7)

AUTHORS
     bhyve-mcp was written for the CloudBSD project.
```

### 11.2 bhyve-mcp-tools.7

This man page documents all MCP tools for AI agent consumption:

```
BHYVE-MCP-TOOLS(7)      FreeBSD Miscellaneous Information Manual     BHYVE-MCP-TOOLS(7)

NAME
     bhyve-mcp-tools -- MCP tool reference for bhyve-mcp

DESCRIPTION
     This document describes the tools exposed by bhyve-mcp(8) via the Model
     Context Protocol. AI agents can discover these tools dynamically via the
     MCP tools/list endpoint.

VM LIFECYCLE TOOLS
     vm_list
          List all VMs and their current status.
          Returns: Array of {name, status, cpu, memory, uptime}.

     vm_create {name, cpu, memory, ...}
          Create a new VM configuration and kernel object.
          Returns: {success, name, config_path}.

     vm_start {name}
          Start a VM. Forks bhyve(8) for device emulation.
          Returns: {success, pid}.

     vm_stop {name}
          Gracefully stop a VM.
          Returns: {success}.

     vm_force_stop {name}
          Forcefully destroy a VM.
          Returns: {success}.

     vm_destroy {name}
          Delete a VM and all associated resources.
          Returns: {success}.

CONSOLE TOOLS
     vm_screenshot {name, format="png"}
          Capture the VM screen via VNC framebuffer.
          Returns: {image_base64, width, height}.

     vm_send_keys {name, keys}
          Send keystrokes to the VM.
          Returns: {success}.

     vm_console_read {name, lines=100}
          Read recent serial console output.
          Returns: {lines: [...]}.

     vm_send_text {name, text}
          Send text to the serial console.
          Returns: {success}.

STORAGE TOOLS
     disk_create {vm_name, type, size, ...}
          Create a disk image.
          Returns: {success, path}.

     disk_delete {vm_name, path}
          Delete a disk image.
          Returns: {success}.

NETWORKING TOOLS
     net_switch_list
          List virtual switches.
          Returns: {switches: [...]}.

     net_switch_create {name, iface}
          Create a virtual switch.
          Returns: {success}.

HOST TOOLS
     host_info
          Get FreeBSD host information.
          Returns: {cpu_count, memory_total, bhyve_supported}.

SEE ALSO
     bhyve-mcp(8), bhyve-mcp.conf(5), bhyve-mcp-vm(5), mcp(7)
```

---

## 12. Multi-Agent Compatibility

### 12.1 Junie

Junie spawns MCP servers as subprocesses. Configuration in `.junie/settings.json`:

```json
{
  "mcpServers": {
    "bhyve": {
      "command": "/usr/local/bin/bhyve-mcp",
      "args": ["--config", "/usr/local/etc/bhyve-mcp/config.yaml"],
      "transport": "stdio"
    }
  }
}
```

### 12.2 OpenCode

OpenCode uses a similar MCP configuration. The server must:
- Support stdio transport.
- Emit clear error messages.
- Provide tool descriptions that are self-documenting.

### 12.3 Claude / Other Agents

Any MCP-compliant client can use bhyve-mcp. The server advertises tools via `tools/list` and handles invocations via `tools/call`.

### 12.4 Agent-Agnostic Design Principles

1. **Self-describing tools**: Tool names and descriptions must be clear without external docs.
2. **Structured errors**: Always return JSON error objects, not plain text.
3. **Idempotent operations**: `vm_create` on existing VM returns error, not crash.
4. **Stateless where possible**: VM state is derived from `libvmmapi`, not cached.
5. **Logging**: Emit `notifications/logging/message` for all significant events.

---

## 13. Security

### 13.1 Privilege Model

- Run as dedicated `bhyve-mcp` user.
- Add to `operator` group for `/dev/vmm/*` access.
- Use `devfs` rules for VM device nodes.
- SSE transport binds to `127.0.0.1` by default.

### 13.2 Resource Limits

- Enforce `vm.max_user_wired` sysctl.
- Configurable `max_vms`, `max_cpu_per_vm`, `max_memory_per_vm`.
- Rate-limit screenshot requests.

### 13.3 Input Sanitization

- Validate all VM names (alphanumeric, hyphen, underscore).
- Reject path traversal in disk paths.
- Sanitize VNC key input.

---

## 14. Testing Strategy

### 14.1 Unit Tests

- Mock `libvmmapi` with a test double.
- Test configuration parsing.
- Test MCP tool routing.

### 14.2 Integration Tests

- Create and destroy VMs in a test environment.
- Verify VNC screenshot capture.
- Test serial console I/O.

### 14.3 Performance Tests

- Measure VM start/stop latency.
- Benchmark screenshot capture FPS.
- Stress test with multiple concurrent VMs.

---

## 15. TODO — Step-by-Step Implementation Tracker

This section is the master checklist for implementing bhyve-mcp. Each task includes:
- **Status:** `NOT STARTED` | `IN PROGRESS` | `COMPLETED`
- **Owner:** Who is working on it
- **Start Date:** When work began
- **End Date:** When work finished
- **Dependencies:** What must be done first
- **Files Modified:** What files are touched
- **Notes:** Any blockers, decisions, or context

### Phase 0: Foundation and Setup

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 0.1 | Create GitHub repository `cloudbsdorg/bhyve-mcp` | NOT STARTED | | | | | | Initialize with README, LICENSE |
| 0.2 | Set up Go module and project structure | NOT STARTED | | | | 0.1 | `go.mod`, `Makefile`, `cmd/`, `internal/` | Follow CloudBSD guidelines |
| 0.3 | Create `.plan` directory with design documents | COMPLETED | | 2026-04-24 | 2026-04-24 | | `.plan/*.md` | This document and supporting plans |
| 0.4 | Verify libvmmapi headers and library exist on target | COMPLETED | | 2026-04-24 | 2026-04-24 | | `/usr/include/vmmapi.h`, `/usr/lib/libvmmapi.so` | Confirmed on FreeBSD 14.x |
| 0.5 | Set up bhyve test VM environment | NOT STARTED | | | | 0.2 | | Need nested bhyve or dedicated host |
| 0.6 | Document baseline host capabilities | NOT STARTED | | | | 0.5 | | CPU, memory, bhyve feature flags |

### Phase 1: Core Infrastructure

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 1.1 | Implement CGO bindings for libvmmapi core | NOT STARTED | | | | 0.2 | `internal/vmmapi/` | vm_create, vm_open, vm_destroy, vm_close |
| 1.2 | Implement CGO bindings for vCPU control | NOT STARTED | | | | 1.1 | `internal/vmmapi/` | vm_vcpu_open, vm_set/get_register, vcpu_reset |
| 1.3 | Implement CGO bindings for VM statistics | NOT STARTED | | | | 1.1 | `internal/vmmapi/` | vm_get_stats, vm_get_stat_desc, vm_active_cpus |
| 1.4 | Implement CGO bindings for interrupts | NOT STARTED | | | | 1.1 | `internal/vmmapi/` | vm_inject_exception, vm_lapic_irq, vm_inject_nmi |
| 1.5 | Create error mapping layer (C errno → Go error) | NOT STARTED | | | | 1.1 | `internal/vmmapi/errors.go` | Map ENOENT, EEXIST, EBUSY, ENOMEM, EPERM |
| 1.6 | Implement configuration parser (YAML) | NOT STARTED | | | | 0.2 | `internal/config/` | Global config + per-VM config |
| 1.7 | Implement state persistence (SQLite or JSON) | NOT STARTED | | | | 1.6 | `internal/store/` | VM state, runtime info |
| 1.8 | Implement logging framework | NOT STARTED | | | | 0.2 | `internal/log/` | Structured logs, syslog support |
| 1.9 | Write unit tests for vmmapi bindings | NOT STARTED | | | | 1.5 | `internal/vmmapi/*_test.go` | Mock libvmmapi where possible |
| 1.10 | Write unit tests for config parser | NOT STARTED | | | | 1.6 | `internal/config/*_test.go` | |

### Phase 2: MCP Server and Tools

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 2.1 | Set up MCP server with stdio transport | NOT STARTED | | | | 0.2 | `internal/mcp/server.go` | Use `github.com/mark3labs/mcp-go` |
| 2.2 | Implement `vm_list` tool | NOT STARTED | | | | 1.1, 2.1 | `internal/mcp/tools.go` | List /dev/vmm entries |
| 2.3 | Implement `vm_create` tool | NOT STARTED | | | | 1.1, 2.1 | `internal/mcp/tools.go` | Create config + vm_create() |
| 2.4 | Implement `vm_start` tool | NOT STARTED | | | | 2.3 | `internal/mcp/tools.go` | Fork bhyve with device model |
| 2.5 | Implement `vm_stop` tool | NOT STARTED | | | | 1.1, 2.1 | `internal/mcp/tools.go` | vm_suspend() |
| 2.6 | Implement `vm_force_stop` tool | NOT STARTED | | | | 1.1, 2.1 | `internal/mcp/tools.go` | vm_destroy() |
| 2.7 | Implement `vm_destroy` tool | NOT STARTED | | | | 1.1, 2.1 | `internal/mcp/tools.go` | vm_destroy() + cleanup |
| 2.8 | Implement `vm_status` tool | NOT STARTED | | | | 1.3, 2.1 | `internal/mcp/tools.go` | vm_get_stats() |
| 2.9 | Implement `host_info` tool | NOT STARTED | | | | 2.1 | `internal/mcp/tools.go` | sysctl queries |
| 2.10 | Implement MCP logging notifications | NOT STARTED | | | | 1.8, 2.1 | `internal/mcp/server.go` | Emit logging/message |
| 2.11 | Write integration tests for VM lifecycle | NOT STARTED | | | | 2.7 | `tests/integration/` | Create, start, stop, destroy cycle |

### Phase 3: Storage and Networking

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 3.1 | Implement ZFS zvol creation | NOT STARTED | | | | 0.2 | `internal/disk/zfs.go` | `zfs create -V` wrapper |
| 3.2 | Implement file-based disk creation | NOT STARTED | | | | 0.2 | `internal/disk/file.go` | `truncate` wrapper |
| 3.3 | Implement `disk_create` tool | NOT STARTED | | | | 3.1, 3.2 | `internal/mcp/tools.go` | |
| 3.4 | Implement `disk_delete` tool | NOT STARTED | | | | 3.3 | `internal/mcp/tools.go` | `zfs destroy` or `rm` |
| 3.5 | Implement `disk_list` tool | NOT STARTED | | | | 3.3 | `internal/mcp/tools.go` | |
| 3.6 | Implement TAP interface management | NOT STARTED | | | | 0.2 | `internal/net/tap.go` | `ifconfig tap` wrapper |
| 3.7 | Implement bridge management | NOT STARTED | | | | 3.6 | `internal/net/bridge.go` | `ifconfig bridge` wrapper |
| 3.8 | Implement `net_switch_create` tool | NOT STARTED | | | | 3.7 | `internal/mcp/tools.go` | |
| 3.9 | Implement `net_switch_list` tool | NOT STARTED | | | | 3.7 | `internal/mcp/tools.go` | |
| 3.10 | Implement `net_bridge_attach` tool | NOT STARTED | | | | 3.6, 3.7 | `internal/mcp/tools.go` | |

### Phase 4: Console and Screen Capture

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 4.1 | Implement nmdm serial console ring buffer | NOT STARTED | | | | 0.2 | `internal/console/serial.go` | Read /dev/nmdm-*-A |
| 4.2 | Implement `vm_console_read` tool | NOT STARTED | | | | 4.1 | `internal/mcp/tools.go` | Return ring buffer contents |
| 4.3 | Implement `vm_send_text` tool | NOT STARTED | | | | 4.1 | `internal/mcp/tools.go` | Write to nmdm |
| 4.4 | Integrate VNC client library | NOT STARTED | | | | 0.2 | `internal/console/vnc.go` | Go VNC client package |
| 4.5 | Implement `vm_screenshot` tool | NOT STARTED | | | | 4.4 | `internal/mcp/tools.go` | Capture framebuffer, encode PNG |
| 4.6 | Implement `vm_send_keys` tool | NOT STARTED | | | | 4.4 | `internal/mcp/tools.go` | VNC key events |
| 4.7 | Implement VNC port allocation per VM | NOT STARTED | | | | 4.4 | `internal/console/vnc.go` | Dynamic port from base_port |
| 4.8 | Write integration tests for console I/O | NOT STARTED | | | | 4.3 | `tests/integration/` | Serial read/write cycle |
| 4.9 | Write integration tests for screenshot | NOT STARTED | | | | 4.5 | `tests/integration/` | Verify PNG output |

### Phase 5: FreeBSD Service and Packaging

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 5.1 | Create rc.d service script | NOT STARTED | | | | 0.2 | `configs/bhyve_mcp` | Follow FreeBSD rc.subr style |
| 5.2 | Create example configuration files | NOT STARTED | | | | 1.6 | `configs/config.yaml` | Global + VM example |
| 5.3 | Create FreeBSD port Makefile | NOT STARTED | | | | 5.1, 5.2 | `ports/sysutils/bhyve-mcp/Makefile` | |
| 5.4 | Create pkg-descr | NOT STARTED | | | | 5.3 | `ports/sysutils/bhyve-mcp/pkg-descr` | Under 80 chars/line |
| 5.5 | Create pkg-plist | NOT STARTED | | | | 5.3 | `ports/sysutils/bhyve-mcp/pkg-plist` | |
| 5.6 | Create port rc.d template | NOT STARTED | | | | 5.1 | `ports/sysutils/bhyve-mcp/files/bhyve_mcp.in` | SUB_LIST macros |
| 5.7 | Test port with `make package` | NOT STARTED | | | | 5.6 | | In poudriere or local |
| 5.8 | Test port with `make install` | NOT STARTED | | | | 5.7 | | Verify files installed |
| 5.9 | Run `portlint` and fix warnings | NOT STARTED | | | | 5.8 | | Must pass cleanly |

### Phase 6: Documentation and Man Pages

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 6.1 | Write `bhyve-mcp.8` man page | NOT STARTED | | | | 2.1 | `docs/bhyve-mcp.8` | Daemon usage, flags |
| 6.2 | Write `bhyve-mcp.conf.5` man page | NOT STARTED | | | | 1.6 | `docs/bhyve-mcp.conf.5` | Global config format |
| 6.3 | Write `bhyve-mcp-vm.5` man page | NOT STARTED | | | | 1.6 | `docs/bhyve-mcp-vm.5` | VM config format |
| 6.4 | Write `bhyve-mcp-tools.7` man page | NOT STARTED | | | | 2.9 | `docs/bhyve-mcp-tools.7` | MCP tool reference |
| 6.5 | Write README.md with usage examples | NOT STARTED | | | | 6.1 | `README.md` | Quick start, agent config |
| 6.6 | Write CONTRIBUTING.md | NOT STARTED | | | | 0.1 | `CONTRIBUTING.md` | CloudBSD standards |
| 6.7 | Install man pages via port | NOT STARTED | | | | 6.1–6.4 | `ports/sysutils/bhyve-mcp/Makefile` | MAN8, MAN5, MAN7 |

### Phase 7: Testing and Hardening

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 7.1 | Implement resource limit enforcement | NOT STARTED | | | | 1.6 | `internal/vm/limits.go` | max_vms, max_cpu, max_mem |
| 7.2 | Implement input sanitization | NOT STARTED | | | | 2.1 | `internal/mcp/validate.go` | VM names, paths |
| 7.3 | Implement rate limiting for screenshots | NOT STARTED | | | | 4.5 | `internal/console/vnc.go` | Per-VM rate limiter |
| 7.4 | Run stress test with 10 concurrent VMs | NOT STARTED | | | | 7.1 | `tests/stress/` | |
| 7.5 | Run memory leak check | NOT STARTED | | | | 7.4 | | valgrind or Go pprof |
| 7.6 | Security audit: file permissions | NOT STARTED | | | | 5.2 | | Verify /dev/vmm access |
| 7.7 | Security audit: network binding | NOT STARTED | | | | 2.1 | | SSE only on 127.0.0.1 |

### Phase 8: Release and Submission

| # | Task | Status | Owner | Start | End | Dependencies | Files | Notes |
|---|------|--------|-------|-------|-----|--------------|-------|-------|
| 8.1 | Tag v0.1.0 release | NOT STARTED | | | | 7.7 | `git tag v0.1.0` | |
| 8.2 | Create GitHub release with notes | NOT STARTED | | | | 8.1 | | |
| 8.3 | Submit port to FreeBSD bugzilla | NOT STARTED | | | | 5.9, 6.7 | | Attach shar or git URL |
| 8.4 | Announce on CloudBSD channels | NOT STARTED | | | | 8.3 | | |
| 8.5 | Publish agent integration guide | NOT STARTED | | | | 6.5 | `docs/AGENTS.md` | Junie, OpenCode, Claude |

---

## 16. Future Enhancements

- **PCI Passthrough**: Expose `vm_assign_pptdev` via MCP.
- **Live Snapshots**: Use `vm_snapshot_req` for VM checkpointing.
- **Cloud-init Integration**: Auto-generate cloud-init ISOs for Linux VMs.
- **REST API**: HTTP REST layer alongside MCP for non-agent clients.
- **WebSocket Console**: Browser-based VNC/serial console.
- **ZFS Snapshot Management**: Expose `zfs snapshot` for VM disks.
- **Migration Helpers**: Export/import VM configurations.

---

## 17. Conclusion

bhyve-mcp bridges the gap between AI coding agents and FreeBSD bhyve virtualization. By using direct `libvmmapi` integration instead of shell wrappers, it provides reliable, performant, and observable VM management. The MCP protocol ensures compatibility with Junie, OpenCode, Claude, and future agents.

The phased implementation approach minimizes risk:
- Phase 0–1 establishes the foundation (CGO bindings, config, tests).
- Phase 2 delivers core MCP tools (VM lifecycle).
- Phase 3–4 add storage, networking, and console capabilities.
- Phase 5–6 ensure production readiness (service, ports, docs).
- Phase 7–8 harden and release.

This plan aligns with CloudBSD's standards for system software: native FreeBSD integration, thorough documentation, man pages, and ports-quality packaging.
