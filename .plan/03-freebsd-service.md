# FreeBSD Service Integration

## rc.d Service Script

Location: `/usr/local/etc/rc.d/bhyve_mcp`

### Requirements

- Start after `NETWORKING` and `kldload vmm` (if not already loaded).
- Run as a dedicated user (`bhyve-mcp` or `root` if required for `vmm` access).
- Log to `/var/log/bhyve-mcp.log` via `syslog` or direct file.
- Support `start`, `stop`, `restart`, `status`.

### rc.conf Variables

```sh
bhyve_mcp_enable="YES"
bhyve_mcp_user="bhyve-mcp"
bhyve_mcp_group="bhyve-mcp"
bhyve_mcp_config="/usr/local/etc/bhyve-mcp/config.yaml"
bhyve_mcp_transport="stdio"      # or "sse"
bhyve_mcp_port="8080"            # for SSE mode
```

### Service Script Skeleton

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
: ${bhyve_mcp_config:="/usr/local/etc/bhyve-mcp/config.yaml"}

pidfile="/var/run/${name}.pid"
command="/usr/local/bin/bhyve-mcp"
command_args="--config ${bhyve_mcp_config}"

run_rc_command "$1"
```

## User & Permissions

- Create user `bhyve-mcp` and group `bhyve-mcp`.
- Add user to `operator` group if needed for `/dev/vmm/*` access.
- Ensure `/dev/vmm` is accessible.
- Consider `devfs` rules for VM device nodes.

## Kernel Modules

The service should ensure these are loaded:

- `vmm` — bhyve kernel module
- `nmdm` — null-modem for serial consoles
- `if_bridge` — networking bridges
- `if_tap` — tap interfaces for VMs

## Logging

- Use `syslog` with facility `local0`.
- Log file: `/var/log/bhyve-mcp.log`.
- Console logs: `/var/log/bhyve-mcp/console/<vm_name>.log` (if `console.persist` is enabled).
- Rotate via `newsyslog`.

## Security

- Run as non-root where possible.
- Use `mac_portacl` or `mac_seeotheruids` if needed.
- Restrict SSE transport to localhost by default (`127.0.0.1`).
- No authentication for stdio transport (relies on process isolation).
