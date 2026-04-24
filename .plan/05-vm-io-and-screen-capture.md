# VM I/O and Screen Capture

## Problem

Junie needs to see what is on the VM's screen and potentially send keyboard/mouse input. This is critical for:
- Debugging boot failures
- Interacting with OS installers
- Verifying VM state when SSH/network is unavailable
- Automated UI testing

## Approaches

### 1. VNC Framebuffer (fbuf)

bhyve supports a VNC-compatible framebuffer device:

```sh
bhyve -s 29,fbuf,tcp=0.0.0.0:5900,w=1024,h=768,wait \
      -s 30,xhci,tablet \
      ...
```

**Pros:**
- Native bhyve support
- Standard VNC protocol — many libraries available
- Can capture screenshots and send input

**Cons:**
- Requires a TCP port per VM (or dynamic allocation)
- Slight performance overhead
- Need a VNC client library in the MCP server

**MCP Tools:**
- `vm_screenshot(vm_name, format="png")` → returns base64 image
- `vm_send_keys(vm_name, keys)` → sends keystrokes
- `vm_send_mouse(vm_name, x, y, buttons)` → sends mouse events

### 2. Serial Console (nmdm / stdio)

For text-mode VMs or serial-enabled guests:

```sh
bhyve -l com1,/dev/nmdm-ubuntu-vm-A \
      -l com2,stdio \
      ...
```

**Pros:**
- Very low overhead
- Works for text-mode installers (FreeBSD, Linux text mode)
- Can stream continuously

**Cons:**
- No graphics — only text
- Requires guest OS to redirect console to serial

**MCP Tools:**
- `vm_console_read(vm_name, lines=100)` → returns recent console output
- `vm_console_send(vm_name, text)` → sends text to serial console

### 3. Bhyve P9 / VirtFS (for file exchange)

For getting files in/out without network:

```sh
bhyve -s 31,virtio-9p,sharename=/host/path ...
```

**Pros:**
- Direct file sharing
- No network required

**Cons:**
- Requires guest driver support

**MCP Tools:**
- `vm_file_push(vm_name, host_path, guest_path)`
- `vm_file_pull(vm_name, guest_path, host_path)`

## Recommended Hybrid Strategy

| Scenario | Method |
|----------|--------|
| Boot debugging / installers | VNC framebuffer + screenshot tool |
| Headless servers | Serial console (nmdm) |
| File transfer | VirtFS or network |
| Automated interaction | VNC for GUI, serial for text |

## Implementation Notes

### VNC Screenshot Capture

Use a VNC client library (e.g., `libvncclient` in C, `vncdotool` in Python, or a Go VNC package):

1. Connect to `localhost:VM_PORT`.
2. Request full framebuffer update.
3. Encode as PNG.
4. Return base64 string in MCP tool result.

### Input Injection

For VNC:
- Map key names to X11 keysyms.
- Send `KeyEvent` messages.
- Support special keys: `Enter`, `Tab`, `Escape`, `F1`–`F12`, etc.

For serial:
- Write raw bytes to `/dev/nmdm-<vm>-A`.
- Handle line endings (`\r\n` vs `\n`).

## Streaming Console Access

Beyond one-shot read/write, Junie needs continuous, real-time access to VM console output. This is essential for:
- Watching boot progress
- Interacting with live installers
- Debugging kernel panics
- Running long commands and seeing output stream

### Streaming Architecture

```
+--------+     MCP Tool      +-------------+     Ring Buffer     +-------------+
| Junie  |  <------------->  | bhyve-mcp   |  <--------------->  | /dev/nmdm   |
| (any)  |   poll or SSE     | (Go)        |   (per-VM buffer)   | (kernel)    |
+--------+                   +-------------+                     +-------------+
       ^                            |
       |                            | WebSocket / SSE
       |                            v
       |                     +-------------+
       |                     | Browser /   |
       +-------------------->| Remote CLI  |
                             +-------------+
```

### Implementation Strategies

#### 1. Polling with Ring Buffer (MCP-native)

Since MCP stdio transport is request/response, use a ring buffer with cursor-based polling:

```yaml
# MCP tool: vm_console_stream
name: "vm_console_stream"
arguments:
  vm_name: "ubuntu-vm"
  cursor: 0                    # byte offset into stream; 0 = start from current end
  timeout_ms: 5000             # max time to wait for new data
  max_lines: 100               # max lines to return
```

**Response:**
```json
{
  "lines": [
    "[    0.000000] Linux version 6.8.0...",
    "[    0.004000] Command line: BOOT_IMAGE=/boot/vmlinuz...",
    "..."
  ],
  "cursor": 1536,               # new byte offset for next poll
  "eof": false,                # true if VM stopped
  "timestamp": "2026-04-24T10:15:30Z"
}
```

**Ring Buffer Design:**
- Per-VM circular buffer in memory (configurable size, default 1 MiB).
- Separate goroutine reads from `/dev/nmdm-<vm>-A` and writes to buffer.
- Cursor is absolute byte offset; clients resume from last cursor.
- Buffer wraps around; old data is overwritten. Clients must poll frequently enough.

#### 2. SSE Transport for Real-Time Streaming

For daemonized mode with SSE transport, push console lines as events:

```
event: console
data: {"vm_name":"ubuntu-vm","line":"[OK] Started sshd.","timestamp":"..."}

id: 42
```

**Benefits:**
- No polling overhead.
- Multiple clients can subscribe to same VM console.
- Natural fit for browser-based or remote CLI access.

#### 3. WebSocket Console (Future)

For non-MCP clients (browsers, remote terminals):

```
ws://host:8080/v1/console/ubuntu-vm
```

- Bidirectional: send keystrokes, receive output.
- Supports ANSI escape sequences for color.
- Can be fronted by a simple web terminal (e.g., `xterm.js`).

### Console Log Persistence

Optionally persist console output to disk for post-mortem analysis:

```yaml
# /usr/local/etc/bhyve-mcp/config.yaml
console:
  persist: true
  log_dir: /var/log/bhyve-mcp/console
  max_log_size: 100M            # per-VM log rotation
  max_log_files: 10             # number of rotated files to keep
```

**File naming:** `/var/log/bhyve-mcp/console/<vm_name>.log`

**Rotation:**
- Rotate when size exceeds `max_log_size`.
- Compress old logs with `gzip`.
- Delete logs when VM is destroyed (configurable).

### Console Access Security

1. **Access Control**: Only the VM owner (or `operator` group) can read console.
2. **Sanitization**: Strip control characters that could corrupt terminal (`\x00`–`\x08`).
3. **Rate Limiting**: Limit `vm_console_stream` calls to 10/sec per VM to prevent CPU exhaustion.
4. **Audit Logging**: Log all console access with timestamp and client ID.

### MCP Tool Additions

| Tool | Description |
|------|-------------|
| `vm_screenshot` | Capture VM screen as base64 PNG |
| `vm_send_keys` | Send keystrokes to VM |
| `vm_send_text` | Send text to serial console |
| `vm_console_read` | Read recent serial console output (one-shot) |
| `vm_console_stream` | Stream console output with cursor-based polling |
| `vm_console_logs` | Retrieve persisted console logs |
| `vm_file_push` | Push file into VM via VirtFS |
| `vm_file_pull` | Pull file from VM via VirtFS |
