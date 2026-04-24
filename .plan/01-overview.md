# bhyve-mcp: Overview & Goals

## Problem Statement

Junie (and other AI agents) running in Linux-compatible environments struggle to manage FreeBSD bhyve virtual machines directly. The Linux compatibility layer gets in the way when trying to use tools like `vm-bhyve` or native `bhyve`/`bhyvectl` commands. We need a pure FreeBSD-native MCP server that exposes VM management capabilities to Junie over the Model Context Protocol.

## Goals

1. **Pure FreeBSD**: Runs natively on FreeBSD without Linux compatibility interference.
2. **MCP Compliant**: Implements the Model Context Protocol so Junie can discover and invoke tools.
3. **VM Lifecycle Management**: Create, start, stop, destroy, and configure bhyve VMs.
4. **Networking & Storage**: Manage virtual switches, bridges, ZFS volumes, and disk images.
5. **ISO & Image Management**: Download, verify, and organize OS installation media and disk images.
6. **Template System**: Create golden master images for rapid VM cloning.
7. **Observability**: Query VM status, logs, console streams, and resource usage.
8. **Service Integration**: Runs as a FreeBSD rc.d service for reliability.

## Non-Goals

- Cross-platform support (Linux, macOS, etc.)
- GUI or web interface
- Live migration (bhyve does not support this natively)
- PCI passthrough configuration (out of scope for initial version)

## Architecture at a Glance

```
+--------+     MCP (stdio/sse)     +-----------+     bhyve/bhyvectl/vm     +--------+
| Junie  |  <------------------->  | bhyve-mcp |  <--------------------->  | bhyve  |
| (any)  |     JSON-RPC tools      | (FreeBSD) |     CLI / libvmmapi      | VMs    |
+--------+                         +-----------+                          +--------+
```

## Key Decisions

- **Transport**: stdio (simplest for Junie integration) and optionally SSE for remote use.
- **Language**: Python 3 (available in FreeBSD ports, easy MCP SDK usage) or Go (static binary, fast). Decision TBD.
- **VM Management Style**: Wrap `bhyve`/`bhyvectl` directly rather than depending on `vm-bhyve` to minimize external dependencies.
- **Configuration**: JSON/YAML files in `/usr/local/etc/bhyve-mcp/`.
- **State Storage**: SQLite or JSON files in `/var/db/bhyve-mcp/`.
