# bhyve-mcp

A Model Context Protocol (MCP) server for FreeBSD that enables AI coding agents to create, manage, and monitor bhyve virtual machines directly via the native `libvmmapi` C library.

## Overview

`bhyve-mcp` bridges the gap between AI coding agents (Junie, OpenCode, Claude, etc.) and FreeBSD bhyve virtualization. By using direct `libvmmapi` integration instead of shell wrappers, it provides reliable, performant, and observable VM management.

## Features

- **Pure FreeBSD**: Runs natively without Linux compatibility.
- **MCP Compliant**: Implements Model Context Protocol for tool discovery.
- **Direct Kernel Integration**: Uses `libvmmapi` for VM lifecycle.
- **Multi-Agent Support**: Works with Junie, OpenCode, Claude, and any MCP client.
- **VM Lifecycle**: Create, configure, start, stop, destroy VMs.
- **Screen Capture**: VNC framebuffer screenshots for visual feedback.
- **Console I/O**: Serial console read/write for text-mode interaction.
- **Storage**: ZFS zvol and file-based disk management.
- **Networking**: Virtual switch and bridge management.
- **Service Integration**: FreeBSD rc.d service with proper logging.
- **Ports Quality**: Submittable to FreeBSD Ports Collection.

## Documentation

See the [`.plan/`](.plan/) directory for detailed implementation plans:

- [`00-master-plan.md`](.plan/00-master-plan.md) — Master implementation plan with TODO tracker
- [`01-overview.md`](.plan/01-overview.md) — Overview and goals
- [`02-mcp-server-design.md`](.plan/02-mcp-server-design.md) — MCP server design
- [`03-freebsd-service.md`](.plan/03-freebsd-service.md) — FreeBSD service integration
- [`04-bhyve-vm-management.md`](.plan/04-bhyve-vm-management.md) — bhyve VM management
- [`05-vm-io-and-screen-capture.md`](.plan/05-vm-io-and-screen-capture.md) — VM I/O and screen capture
- [`06-tech-stack.md`](.plan/06-tech-stack.md) — Tech stack and implementation
- [`07-libvmmapi-integration.md`](.plan/07-libvmmapi-integration.md) — Direct libvmmapi integration
- [`08-freebsd-ports.md`](.plan/08-freebsd-ports.md) — FreeBSD ports packaging

## License

BSD-2-Clause
