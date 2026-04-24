# bhyve VM Management

## VM Configuration Model

Each VM is defined by a configuration file (JSON/YAML) stored in `/usr/local/etc/bhyve-mcp/vms/<name>.yaml`.

### Example VM Config

```yaml
name: ubuntu-vm
cpu: 2
memory: 4096M
boot:
  loader: uefi
  firmware: /usr/local/share/uefi-firmware/BHYVE_UEFI.fd
  disk: hd0
  # or for grub-bhyve:
  # loader: grub
  # device: "(hd0,msdos1)/boot/grub/grub.cfg"
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
  - type: stdio
    # for debugging only
flags:
  - -H              # yield on hlt
  - -w              # ignore accesses to unimplemented MSRs
  - -u              # RTC keeps UTC
```

## VM Lifecycle States

```
[defined] --start--> [starting] --boot--> [running] --stop--> [stopped]
   |                      |                    |                |
   |                      |                    |                |
   +----destroy-----------+--------------------+----------------+
```

## bhyve Command Construction

The server translates VM configs into `bhyve` CLI arguments:

```sh
bhyve -c 2 -m 4096M -H -w -u \
  -s 0,hostbridge \
  -s 1,lpc \
  -s 2,virtio-blk,/dev/zvol/zroot/vm/ubuntu-vm/disk0 \
  -s 3,virtio-net,tap0,mac=00:a0:98:de:ad:01 \
  -s 4,ahci-cd,/var/lib/bhyve-mcp/isos/ubuntu.iso \
  -l bootrom,/usr/local/share/uefi-firmware/BHYVE_UEFI.fd \
  -l com1,/dev/nmdm-ubuntu-vm-A \
  ubuntu-vm
```

## Disk Management

### Supported Backends

| Type | Command | Notes |
|------|---------|-------|
| `zvol` | `zfs create -V size zpool/vm/name/diskN` | Preferred, fast, snapshots |
| `file` | `truncate -s size /path/to/disk.img` | Simple, portable |
| `qcow2` | `qemu-img create -f qcow2 ...` | Requires qemu-tools |

### ISO Management

- Store ISOs in `/var/lib/bhyve-mcp/isos/`.
- Allow `fetch` tool to download ISOs from URLs.

## Networking

### Virtual Switch / Bridge Model

```yaml
switches:
  - name: vmbridge0
    type: bridge
    iface: re0          # physical interface to bridge
    dhcp: true          # run dnsmasq/dhcpd on bridge?
```

### TAP Interface Lifecycle

- Create `tapN` on VM start: `ifconfig tapN create`
- Add to bridge: `ifconfig vmbridge0 addm tapN`
- Destroy on VM stop: `ifconfig tapN destroy`

## Console Access

- **nmdm**: `cu -l /dev/nmdm-<vm>-B` for human access; server reads from `-A`.
- **stdio**: captured by server and exposed via `vm_console` tool.
- **VNC**: optional, via `bhyve -s N,fbuf,tcp=0.0.0.0:5900`.

## UEFI vs GRUB

| Loader | Use Case | Firmware |
|--------|----------|----------|
| UEFI | Modern OSes (Windows, Linux, FreeBSD 13+) | `BHYVE_UEFI.fd` |
| GRUB | Legacy Linux, custom kernels | `grub-bhyve` |

## Resource Limits

- Enforce `vm.max_user_wired` sysctl limits.
- Track per-VM CPU/memory usage via `libvmmapi` or `top` parsing.
