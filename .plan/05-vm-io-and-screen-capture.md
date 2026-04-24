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

## Security Considerations

- VNC ports should bind to `127.0.0.1` only.
- Optionally support VNC password authentication.
- Rate-limit screenshot requests to avoid CPU exhaustion.
- Sanitize all input to prevent injection attacks.

## MCP Tool Additions

Add these tools to the server design:

| Tool | Description |
|------|-------------|
| `vm_screenshot` | Capture VM screen as base64 PNG |
| `vm_send_keys` | Send keystrokes to VM |
| `vm_send_text` | Send text to serial console |
| `vm_console_read` | Read recent serial console output |
| `vm_file_push` | Push file into VM via VirtFS |
| `vm_file_pull` | Pull file from VM via VirtFS |
