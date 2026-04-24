# ISO and Disk Image Management

## Problem

AI agents need a reliable way to obtain, validate, and attach OS installation media and disk images to bhyve VMs. Manually downloading ISOs, verifying checksums, and organizing them is tedious and error-prone. The MCP server must automate this while enforcing storage quotas and security policies.

## Goals

1. **Automated Downloads**: Fetch ISOs and disk images from URLs with resume support.
2. **Integrity Verification**: Validate SHA256/SHA512 checksums after download.
3. **Organized Storage**: Maintain a structured repository of ISOs and templates.
4. **Cloud-init Support**: Auto-generate cloud-init ISOs for unattended Linux VM provisioning.
5. **Disk Image Conversion**: Support raw, QCOW2, and ZFS zvol formats.
6. **Quota Enforcement**: Limit total disk space consumed by images.
7. **Template System**: Maintain golden master images for rapid VM cloning.

---

## Storage Layout

```
/var/lib/bhyve-mcp/
├── isos/                    # Installation media
│   ├── FreeBSD-14.0-RELEASE-amd64-disc1.iso
│   ├── ubuntu-24.04-live-server-amd64.iso
│   └── checksums.sha256     # Central checksum database
├── disks/                   # VM data disks (file-based)
│   ├── ubuntu-vm-disk0.raw
│   └── windows-vm-disk0.qcow2
├── templates/               # Golden master images
│   ├── freebsd-14-base.zvol
│   └── ubuntu-24-base.qcow2
└── cloud-init/              # Auto-generated cloud-init ISOs
    ├── ubuntu-vm-seed.iso
    └── freebsd-vm-seed.iso
```

---

## ISO Management

### Downloading ISOs

The MCP server provides a tool to download ISOs from URLs:

```yaml
# MCP tool: iso_download
name: "iso_download"
arguments:
  url: "https://download.freebsd.org/.../FreeBSD-14.0-RELEASE-amd64-disc1.iso"
  filename: "FreeBSD-14.0-RELEASE-amd64-disc1.iso"
  checksum: "sha256:abc123..."      # optional but recommended
  verify: true                      # fail if checksum mismatch
```

**Implementation:**
- Use `fetch(1)` or Go's `net/http` with resume support (`Range` headers).
- Store partial downloads as `.part` files, rename on completion.
- Verify checksum before marking as available.
- Enforce `limits.max_iso_size` (default: 10 GiB per ISO).

### Checksum Database

Maintain a JSON/YAML database of known-good ISOs:

```yaml
# /var/db/bhyve-mcp/iso-db.yaml
isos:
  - name: "FreeBSD-14.0-RELEASE-amd64-disc1.iso"
    url: "https://download.freebsd.org/..."
    sha256: "a1b2c3d4..."
    size: 1234567890
    downloaded: "2026-04-24T09:00:00Z"
    verified: true
```

### ISO Listing and Cleanup

```yaml
# MCP tool: iso_list
name: "iso_list"
arguments:
  filter: "FreeBSD*"          # glob pattern

# MCP tool: iso_delete
name: "iso_delete"
arguments:
  filename: "FreeBSD-14.0-RELEASE-amd64-disc1.iso"
  force: false                # fail if attached to any VM config
```

**Safety:**
- Prevent deletion of ISOs referenced in active VM configs.
- `force: true` overrides with explicit confirmation.

---

## Disk Image Management

### Supported Formats

| Format | Extension | Creation | Resize | Notes |
|--------|-----------|----------|--------|-------|
| Raw | `.raw`, `.img` | `truncate` | `truncate` | Simple, no overhead |
| QCOW2 | `.qcow2` | `qemu-img create` | `qemu-img resize` | Sparse, snapshots |
| ZFS zvol | n/a (ZFS path) | `zfs create -V` | `zfs set volsize` | Preferred on ZFS |

### Disk Image Lifecycle

```
[create] --attach--> [attached to VM] --detach--> [detached]
   |                      |                           |
   |                      |                           |
   +----resize------------+---------------------------+
   |                      |                           |
   +----clone-------------+---------------------------+
   |                                                  |
   +----destroy---------------------------------------+
```

### MCP Tools

```yaml
# disk_create
name: "disk_create"
arguments:
  vm_name: "ubuntu-vm"
  type: "zvol"                    # zvol | file | qcow2
  size: "20G"
  pool: "zroot"                   # for zvol
  path: "/var/lib/bhyve-mcp/disks/ubuntu-vm-disk0.raw"  # for file/qcow2

# disk_resize
name: "disk_resize"
arguments:
  vm_name: "ubuntu-vm"
  path: "zroot/vm/ubuntu-vm/disk0"
  new_size: "40G"
  allow_shrink: false           # safety guard

# disk_clone
name: "disk_clone"
arguments:
  source: "zroot/vm/ubuntu-vm/disk0"
  dest: "zroot/vm/ubuntu-vm-clone/disk0"
  type: "zfs_snapshot"          # zfs_snapshot | zfs_send | file_copy

# disk_delete
name: "disk_delete"
arguments:
  vm_name: "ubuntu-vm"
  path: "zroot/vm/ubuntu-vm/disk0"
  force: false                  # fail if VM is running
```

---

## Cloud-init Integration

For unattended Linux VM provisioning, generate cloud-init ISOs:

### Configuration

```yaml
# cloud-init config passed via MCP
ci_user_data: |
  #cloud-config
  users:
    - name: admin
      sudo: ALL=(ALL) NOPASSWD:ALL
      ssh_authorized_keys:
        - ssh-rsa AAAA...
  package_update: true
  packages:
    - curl
    - git

ci_meta_data: |
  instance-id: ubuntu-vm-001
  local-hostname: ubuntu-vm
```

### ISO Generation

```sh
# Create temporary directory
mkdir -p /tmp/ci-ubuntu-vm

# Write user-data and meta-data
cat > /tmp/ci-ubuntu-vm/user-data <<EOF
#cloud-config
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
EOF

cat > /tmp/ci-ubuntu-vm/meta-data <<EOF
instance-id: ubuntu-vm-001
local-hostname: ubuntu-vm
EOF

# Generate ISO
mkisofs -o /var/lib/bhyve-mcp/cloud-init/ubuntu-vm-seed.iso \
  -V cidata -J -r /tmp/ci-ubuntu-vm/
```

**MCP Tool:**
```yaml
name: "iso_cloudinit_create"
arguments:
  vm_name: "ubuntu-vm"
  user_data: "#cloud-config\nusers:\n  - name: admin..."
  meta_data: "instance-id: ubuntu-vm-001..."
  network_config: ""            # optional
```

### VM Attachment

Attach cloud-init ISO as a second CD-ROM:

```sh
bhyve -s 0,hostbridge \
  -s 1,lpc \
  -s 2,virtio-blk,/dev/zvol/zroot/vm/ubuntu-vm/disk0 \
  -s 3,ahci-cd,/var/lib/bhyve-mcp/isos/ubuntu-24.04-live-server-amd64.iso \
  -s 4,ahci-cd,/var/lib/bhyve-mcp/cloud-init/ubuntu-vm-seed.iso \
  ...
```

---

## Template System

### Golden Master Images

Create pre-installed OS images for rapid cloning:

```yaml
# MCP tool: template_create
name: "template_create"
arguments:
  template_name: "ubuntu-24-base"
  source_vm: "ubuntu-installer"
  description: "Ubuntu 24.04 LTS with base packages"
  tags: ["ubuntu", "lts", "base"]
```

**Implementation:**
1. Install VM from ISO with automated kickstart/preseed/cloud-init.
2. Shut down VM.
3. Snapshot/clone the disk as a template.
4. Store metadata in `/var/db/bhyve-mcp/templates.yaml`.

### Template Cloning

```yaml
# MCP tool: vm_create_from_template
name: "vm_create_from_template"
arguments:
  vm_name: "ubuntu-dev-01"
  template: "ubuntu-24-base"
  disk_size: "40G"              # optional: expand beyond template
  cpu: 4
  memory: "8192M"
```

**Implementation:**
- ZFS: `zfs clone zroot/templates/ubuntu-24-base@gold zroot/vm/ubuntu-dev-01/disk0`
- File/QCOW2: `qemu-img create -b template.qcow2 -f qcow2 new.qcow2`
- Expand if requested: `zfs set volsize=...` or `qemu-img resize`

---

## Resource Limits and Quotas

### Per-Category Limits

```yaml
# /usr/local/etc/bhyve-mcp/config.yaml
limits:
  max_iso_storage: 50G          # total for /var/lib/bhyve-mcp/isos
  max_disk_storage: 500G        # total for /var/lib/bhyve-mcp/disks
  max_template_storage: 200G    # total for /var/lib/bhyve-mcp/templates
  max_iso_size: 10G             # per-ISO limit
  max_disk_size: 100G           # per-disk limit
```

### Enforcement

- Before any download/create/clone operation, check current usage vs. limit.
- Reject operations that would exceed limits with clear MCP error:
  ```json
  {
    "error": "insufficient_storage",
    "message": "ISO download would exceed max_iso_storage (50G). Current: 48.2G, Requested: 4.7G.",
    "limit": "50G",
    "current": "48.2G",
    "requested": "4.7G"
  }
  ```

### Cleanup Policies

```yaml
# Automatic cleanup of old/unused images
cleanup:
  iso_max_age: 90d              # delete ISOs not used in 90 days
  template_max_age: 365d       # delete templates not cloned in 365 days
  dry_run: true                 # log what would be deleted without doing it
```

---

## Security Considerations

1. **URL Validation**: Only allow HTTPS URLs. Block `file://`, `ftp://`, etc.
2. **Path Sanitization**: Reject paths with `..`, symlinks, or absolute paths outside `iso_dir`/`disk_dir`.
3. **Checksum Verification**: Mandate checksums for ISOs from external URLs.
4. **Permission Isolation**: Run downloads as `bhyve-mcp` user, not root.
5. **Rate Limiting**: Limit download bandwidth to avoid saturating host network.
6. **Virus Scanning**: Optional integration with `clamav` for downloaded images.

---

## MCP Tool Summary

| Tool | Description |
|------|-------------|
| `iso_download` | Download ISO from URL with optional checksum verification |
| `iso_list` | List available ISOs with metadata |
| `iso_delete` | Delete an ISO (with safety checks) |
| `iso_cloudinit_create` | Generate cloud-init ISO for unattended provisioning |
| `disk_create` | Create a new disk (zvol, file, qcow2) |
| `disk_resize` | Resize an existing disk |
| `disk_clone` | Clone a disk from source to destination |
| `disk_delete` | Delete a disk (with safety checks) |
| `disk_list` | List disks for a VM or globally |
| `template_create` | Create a golden master template from a VM |
| `template_list` | List available templates |
| `template_delete` | Delete a template |
| `vm_create_from_template` | Create a new VM from a template |

---

## Future Enhancements

- **Torrent Downloads**: Support BitTorrent for large ISO distributions.
- **Delta Updates**: Download only changed blocks for updated ISOs.
- **Remote Storage**: S3/MinIO backend for ISO/template storage.
- **Image Registry**: Integration with cloud image registries (AWS AMI, Azure VHD).
- **Live Migration Prep**: Export VM disks to portable formats.
