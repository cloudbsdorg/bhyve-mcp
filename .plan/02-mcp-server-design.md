# MCP Server Design

## Transport

- **Primary**: `stdio` — Junie spawns the MCP server as a subprocess and communicates over stdin/stdout.
- **Secondary**: `SSE` (Server-Sent Events) over HTTP — for remote or daemonized usage.

## Server Capabilities

The server will advertise the following MCP capabilities:

| Capability | Description |
|------------|-------------|
| `tools` | Expose VM management tools to Junie |
| `logging` | Emit structured logs back to Junie |

## Tool Categories

### VM Lifecycle

| Tool | Description |
|------|-------------|
| `vm_list` | List all VMs and their statuses |
| `vm_create` | Create a new VM configuration |
| `vm_start` | Start a VM |
| `vm_stop` | Gracefully stop a VM |
| `vm_force_stop` | Forcefully destroy a VM |
| `vm_destroy` | Delete a VM and its resources |
| `vm_console` | Attach to or read VM console output |

### Storage

| Tool | Description |
|------|-------------|
| `disk_create` | Create a new disk image (raw, zvol, qcow2) |
| `disk_resize` | Resize an existing disk |
| `disk_clone` | Clone a disk from source to destination |
| `disk_delete` | Delete a disk image |
| `disk_list` | List disks for a VM |

### ISO and Image Management

| Tool | Description |
|------|-------------|
| `iso_download` | Download ISO from URL with optional checksum verification |
| `iso_list` | List available ISOs with metadata |
| `iso_delete` | Delete an ISO (with safety checks) |
| `iso_cloudinit_create` | Generate cloud-init ISO for unattended provisioning |
| `template_create` | Create a golden master template from a VM |
| `template_list` | List available templates |
| `template_delete` | Delete a template |
| `vm_create_from_template` | Create a new VM from a template |

### Networking

| Tool | Description |
|------|-------------|
| `net_switch_list` | List virtual switches |
| `net_switch_create` | Create a virtual switch |
| `net_switch_delete` | Delete a virtual switch |
| `net_bridge_attach` | Attach VM NIC to a bridge/switch |

### Host & Observability

| Tool | Description |
|------|-------------|
| `host_info` | Get FreeBSD host info (CPU, memory, bhyve support) |
| `vm_status` | Detailed status of a specific VM |
| `vm_logs` | Retrieve VM boot logs |
| `vm_stats` | CPU/memory/disk stats for a running VM |
| `vm_console_stream` | Stream console output with cursor-based polling |
| `vm_console_logs` | Retrieve persisted console logs |

## Error Handling

All tools return standardized MCP error objects:

- `vm_not_found`
- `vm_already_running`
- `vm_not_running`
- `disk_not_found`
- `iso_not_found`
- `template_not_found`
- `insufficient_resources`
- `insufficient_storage`
- `permission_denied`
- `checksum_mismatch`
- `bhyve_error` (wraps stderr from bhyve commands)

## Logging

The server emits `notifications/logging/message` with levels:
- `debug` — internal state changes
- `info` — VM start/stop events
- `warning` — resource constraints
- `error` — bhyve command failures
