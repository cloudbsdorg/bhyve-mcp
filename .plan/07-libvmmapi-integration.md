# Direct libvmmapi Integration

## Rationale

Shelling out to `bhyve` and `bhyvectl` is fragile, slow, and error-prone. We will use the native `libvmmapi` C library directly via Go's CGO. This gives us:

- **Performance**: No process spawn overhead.
- **Reliability**: Direct error codes, no stderr parsing.
- **Control**: Fine-grained VM state manipulation.
- **Observability**: Direct access to VM statistics and memory.

## Available APIs

### VM Lifecycle (`vmmapi.h`)

| Function | Purpose |
|----------|---------|
| `vm_create(name)` | Create a new VM kernel object |
| `vm_open(name)` | Open existing VM |
| `vm_openf(name, flags)` | Open with flags (create, reinit, destroy-on-close) |
| `vm_close(ctx)` | Close VM handle |
| `vm_destroy(ctx)` | Destroy VM completely |
| `vm_setup_memory(ctx, len, style)` | Allocate guest memory |
| `vm_vcpu_open(ctx, vcpuid)` | Open a vCPU |
| `vm_run(vcpu, vmrun)` | Run a vCPU (the main loop) |
| `vm_suspend(ctx, how)` | Suspend VM |
| `vm_reinit(ctx)` | Reinitialize VM |

### vCPU Control

| Function | Purpose |
|----------|---------|
| `vm_set_register(vcpu, reg, val)` | Set GPR/CR/DR/segment registers |
| `vm_get_register(vcpu, reg, retval)` | Read registers |
| `vcpu_reset(vcpu)` | Reset vCPU state |
| `vm_activate_cpu(vcpu)` | Activate vCPU |
| `vm_suspend_cpu(vcpu)` | Suspend specific vCPU |
| `vm_resume_cpu(vcpu)` | Resume specific vCPU |

### Interrupts & IRQs

| Function | Purpose |
|----------|---------|
| `vm_inject_exception(vcpu, vector, ...)` | Inject exception |
| `vm_lapic_irq(vcpu, vector)` | Send LAPIC IRQ |
| `vm_ioapic_assert_irq(ctx, irq)` | Assert IOAPIC line |
| `vm_isa_assert_irq(ctx, atpic, ioapic)` | Assert ISA IRQ |
| `vm_inject_nmi(vcpu)` | Inject NMI |

### Statistics

| Function | Purpose |
|----------|---------|
| `vm_get_stats(vcpu, tv, entries)` | Get per-vCPU stats array |
| `vm_get_stat_desc(ctx, index)` | Get stat name/description |
| `vm_active_cpus(ctx, cpus)` | Get active CPU set |
| `vm_suspended_cpus(ctx, cpus)` | Get suspended CPU set |

### Memory

| Function | Purpose |
|----------|---------|
| `vm_map_gpa(ctx, gaddr, len)` | Map guest physical address |
| `vm_rev_map_gpa(ctx, addr)` | Reverse map host→guest |
| `vm_get_memseg(ctx, ident, ...)` | Get memory segment info |
| `vm_create_devmem(ctx, segid, name, len)` | Create device memory |
| `vm_mmap_memseg(ctx, gpa, segid, ...)` | Map segment into guest |

### PCI Passthrough (optional)

| Function | Purpose |
|----------|---------|
| `vm_assign_pptdev(ctx, bus, slot, func)` | Assign PCI device |
| `vm_unassign_pptdev(ctx, bus, slot, func)` | Remove PCI device |
| `vm_setup_pptdev_msi(...)` | Setup MSI for passthrough |

## Architecture

```
+-------------+     CGO      +-------------+     ioctl    +-------------+
|  bhyve-mcp  |  <-------->  |  Go wrapper |  <------->  |  libvmmapi |
|   (Go)      |   bindings   |   (cgo)     |   calls    |    (C)      |
+-------------+              +-------------+            +-------------+
                                                              |
                                                              v
                                                        +-------------+
                                                        |   /dev/vmm  |
                                                        |   (kernel)  |
                                                        +-------------+
```

## CGO Binding Strategy

### Option A: Inline CGO

Write CGO directly in Go files:

```go
/*
#include <vmmapi.h>
#include <machine/vmm.h>
*/
import "C"

func vmCreate(name string) error {
    cname := C.CString(name)
    defer C.free(unsafe.Pointer(cname))
    ret := C.vm_create(cname)
    if ret != 0 {
        return fmt.Errorf("vm_create failed: %d", ret)
    }
    return nil
}
```

### Option B: Separate C Wrapper Library

Create a small C library (`libbhyvemcp`) that wraps `libvmmapi` with a simpler API, then bind to that:

```c
// bhyvemcp.h
int bhyvemcp_vm_create(const char *name, int cpus, size_t mem);
int bhyvemcp_vm_destroy(const char *name);
int bhyvemcp_vm_start(const char *name);
int bhyvemcp_vm_stop(const char *name);
```

**Recommended**: Option A for direct control, Option B if the API surface becomes too large.

## What We Still Shell Out For

Some operations don't have `libvmmapi` equivalents and still require subprocesses:

| Operation | Command | Why |
|-----------|---------|-----|
| TAP interface create/destroy | `ifconfig tapN create/destroy` | Network stack |
| Bridge management | `ifconfig bridge0 addm tapN` | Network stack |
| ZFS zvol create/destroy | `zfs create -V ...` | ZFS CLI is the API |
| Disk image creation | `truncate`, `qemu-img` | No direct API |
| UEFI firmware | `bhyve -l bootrom,...` | Firmware loading |
| Device model (virtio-blk, virtio-net, etc.) | `bhyve -s ...` | Device emulation is in userspace bhyve |

**Key insight**: `libvmmapi` controls the VM kernel object (vCPUs, memory, interrupts). The `bhyve` binary provides device emulation (disks, NICs, framebuffer). We use `libvmmapi` for VM lifecycle and `bhyve` for device model.

## Hybrid Approach

1. **VM Create/Destroy**: Use `libvmmapi` (`vm_create`, `vm_destroy`).
2. **VM Start**: Fork `bhyve` process but manage it via `libvmmapi` for status.
3. **VM Stop**: Use `libvmmapi` (`vm_suspend`, `vm_destroy`) or signal `bhyve`.
4. **Stats/Status**: Use `libvmmapi` (`vm_get_stats`, `vm_active_cpus`).
5. **Console/Screen**: Use `bhyve` device model (`-l com1`, `-s fbuf`).

## Thread Safety

`libvmmapi` is NOT MT-safe for statistics (`vm_get_stats`). We must:
- Serialize VM operations per-VM.
- Use a mutex around `vm_get_stats`.
- Each VM gets its own goroutine for the `bhyve` device model process.

## Error Handling

`libvmmapi` returns `int` error codes. Map to Go errors:

```go
var (
    ErrVMNotFound    = errors.New("VM not found")
    ErrVMAlreadyExists = errors.New("VM already exists")
    ErrVMRunning     = errors.New("VM is running")
    ErrVMNotRunning  = errors.New("VM is not running")
    ErrNoMemory      = errors.New("insufficient memory")
    ErrPermission    = errors.New("permission denied")
)

func mapVMError(ret C.int) error {
    switch ret {
    case 0: return nil
    case ENOENT: return ErrVMNotFound
    case EEXIST: return ErrVMAlreadyExists
    case EBUSY: return ErrVMRunning
    case ENOMEM: return ErrNoMemory
    case EPERM: return ErrPermission
    default: return fmt.Errorf("vmmapi error %d: %s", ret, C.GoString(C.strerror(ret)))
    }
}
```
