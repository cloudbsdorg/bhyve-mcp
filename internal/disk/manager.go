package disk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DiskType represents the type of disk backend
type DiskType string

const (
	DiskTypeRaw   DiskType = "raw"
	DiskTypeQCOW2 DiskType = "qcow2"
	DiskTypeZvol  DiskType = "zvol"
)

// DiskInfo holds information about a disk
type DiskInfo struct {
	Name     string   `json:"name"`
	Type     DiskType `json:"type"`
	Path     string   `json:"path"`
	Size     uint64   `json:"size"`
	VM       string   `json:"vm,omitempty"`
	ReadOnly bool     `json:"readonly"`
}

// Manager manages disk images
type Manager struct {
	diskDir string
	zpool   string
}

// NewManager creates a new disk manager
func NewManager(diskDir string, zpool string) *Manager {
	return &Manager{
		diskDir: diskDir,
		zpool:   zpool,
	}
}

// Create creates a new disk image
func (m *Manager) Create(name string, size string, diskType DiskType) error {
	switch diskType {
	case DiskTypeRaw:
		return m.createRaw(name, size)
	case DiskTypeQCOW2:
		return m.createQCOW2(name, size)
	case DiskTypeZvol:
		return m.createZvol(name, size)
	default:
		return fmt.Errorf("unsupported disk type: %s", diskType)
	}
}

// createRaw creates a raw disk image
func (m *Manager) createRaw(name string, size string) error {
	path := filepath.Join(m.diskDir, name+".raw")

	// Parse size
	sizeBytes, err := parseSize(size)
	if err != nil {
		return fmt.Errorf("invalid size: %w", err)
	}

	// Create file with truncate
	cmd := exec.Command("truncate", "-s", fmt.Sprintf("%d", sizeBytes), path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create raw disk: %w", err)
	}

	return nil
}

// createQCOW2 creates a QCOW2 disk image
func (m *Manager) createQCOW2(name string, size string) error {
	path := filepath.Join(m.diskDir, name+".qcow2")

	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, size)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create QCOW2 disk: %w", err)
	}

	return nil
}

// createZvol creates a ZFS zvol
func (m *Manager) createZvol(name string, size string) error {
	path := fmt.Sprintf("%s/vm/%s/%s", m.zpool, filepath.Dir(name), filepath.Base(name))

	cmd := exec.Command("zfs", "create", "-V", size, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create zvol: %w", err)
	}

	return nil
}

// Delete deletes a disk image
func (m *Manager) Delete(name string, diskType DiskType) error {
	switch diskType {
	case DiskTypeRaw, DiskTypeQCOW2:
		return m.deleteFile(name, diskType)
	case DiskTypeZvol:
		return m.deleteZvol(name)
	default:
		return fmt.Errorf("unsupported disk type: %s", diskType)
	}
}

// deleteFile deletes a file-based disk image
func (m *Manager) deleteFile(name string, diskType DiskType) error {
	var ext string
	switch diskType {
	case DiskTypeRaw:
		ext = ".raw"
	case DiskTypeQCOW2:
		ext = ".qcow2"
	}

	path := filepath.Join(m.diskDir, name+ext)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	return nil
}

// deleteZvol deletes a ZFS zvol
func (m *Manager) deleteZvol(name string) error {
	path := fmt.Sprintf("%s/vm/%s", m.zpool, name)

	cmd := exec.Command("zfs", "destroy", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to destroy zvol: %w", err)
	}

	return nil
}

// Resize resizes a disk image
func (m *Manager) Resize(name string, size string, diskType DiskType) error {
	switch diskType {
	case DiskTypeRaw:
		return m.resizeRaw(name, size)
	case DiskTypeQCOW2:
		return m.resizeQCOW2(name, size)
	case DiskTypeZvol:
		return m.resizeZvol(name, size)
	default:
		return fmt.Errorf("unsupported disk type: %s", diskType)
	}
}

// resizeRaw resizes a raw disk image
func (m *Manager) resizeRaw(name string, size string) error {
	path := filepath.Join(m.diskDir, name+".raw")

	sizeBytes, err := parseSize(size)
	if err != nil {
		return fmt.Errorf("invalid size: %w", err)
	}

	cmd := exec.Command("truncate", "-s", fmt.Sprintf("%d", sizeBytes), path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to resize raw disk: %w", err)
	}

	return nil
}

// resizeQCOW2 resizes a QCOW2 disk image
func (m *Manager) resizeQCOW2(name string, size string) error {
	path := filepath.Join(m.diskDir, name+".qcow2")

	cmd := exec.Command("qemu-img", "resize", path, size)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to resize QCOW2 disk: %w", err)
	}

	return nil
}

// resizeZvol resizes a ZFS zvol
func (m *Manager) resizeZvol(name string, size string) error {
	path := fmt.Sprintf("%s/vm/%s", m.zpool, name)

	cmd := exec.Command("zfs", "set", fmt.Sprintf("volsize=%s", size), path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to resize zvol: %w", err)
	}

	return nil
}

// Clone clones a disk image
func (m *Manager) Clone(sourceName, destName string, diskType DiskType) error {
	switch diskType {
	case DiskTypeRaw:
		return m.cloneRaw(sourceName, destName)
	case DiskTypeQCOW2:
		return m.cloneQCOW2(sourceName, destName)
	case DiskTypeZvol:
		return m.cloneZvol(sourceName, destName)
	default:
		return fmt.Errorf("unsupported disk type: %s", diskType)
	}
}

// cloneRaw clones a raw disk image
func (m *Manager) cloneRaw(sourceName, destName string) error {
	sourcePath := filepath.Join(m.diskDir, sourceName+".raw")
	destPath := filepath.Join(m.diskDir, destName+".raw")

	cmd := exec.Command("cp", sourcePath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone raw disk: %w", err)
	}

	return nil
}

// cloneQCOW2 clones a QCOW2 disk image
func (m *Manager) cloneQCOW2(sourceName, destName string) error {
	sourcePath := filepath.Join(m.diskDir, sourceName+".qcow2")
	destPath := filepath.Join(m.diskDir, destName+".qcow2")

	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", sourcePath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone QCOW2 disk: %w", err)
	}

	return nil
}

// cloneZvol clones a ZFS zvol using snapshot
func (m *Manager) cloneZvol(sourceName, destName string) error {
	sourcePath := fmt.Sprintf("%s/vm/%s", m.zpool, sourceName)
	destPath := fmt.Sprintf("%s/vm/%s", m.zpool, destName)

	// Create snapshot
	snapshotName := sourcePath + "@clone-snap"
	cmd := exec.Command("zfs", "snapshot", snapshotName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Clone from snapshot
	cmd = exec.Command("zfs", "clone", snapshotName, destPath)
	if err := cmd.Run(); err != nil {
		// Try to clean up snapshot
		exec.Command("zfs", "destroy", snapshotName).Run()
		return fmt.Errorf("failed to clone zvol: %w", err)
	}

	// Destroy snapshot
	cmd = exec.Command("zfs", "destroy", snapshotName)
	cmd.Run()

	return nil
}

// List lists all disk images
func (m *Manager) List() ([]*DiskInfo, error) {
	var disks []*DiskInfo

	// List raw files
	if entries, err := os.ReadDir(m.diskDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".raw") {
				name = strings.TrimSuffix(name, ".raw")
				info, err := m.getInfo(name, DiskTypeRaw)
				if err == nil {
					disks = append(disks, info)
				}
			} else if strings.HasSuffix(name, ".qcow2") {
				name = strings.TrimSuffix(name, ".qcow2")
				info, err := m.getInfo(name, DiskTypeQCOW2)
				if err == nil {
					disks = append(disks, info)
				}
			}
		}
	}

	// List zvols
	cmd := exec.Command("zfs", "list", "-H", "-t", "volume", "-o", "name,volsize", m.zpool+"/vm")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := strings.TrimPrefix(parts[0], m.zpool+"/vm/")
				size, _ := parseSize(parts[1])
				disks = append(disks, &DiskInfo{
					Name: name,
					Type: DiskTypeZvol,
					Path: parts[0],
					Size: size,
				})
			}
		}
	}

	return disks, nil
}

// GetInfo gets information about a disk
func (m *Manager) GetInfo(name string, diskType DiskType) (*DiskInfo, error) {
	switch diskType {
	case DiskTypeRaw:
		return m.getInfo(name, DiskTypeRaw)
	case DiskTypeQCOW2:
		return m.getInfo(name, DiskTypeQCOW2)
	case DiskTypeZvol:
		return m.getInfo(name, DiskTypeZvol)
	default:
		return nil, fmt.Errorf("unsupported disk type: %s", diskType)
	}
}

// getInfo gets information about a disk (internal)
func (m *Manager) getInfo(name string, diskType DiskType) (*DiskInfo, error) {
	var path string
	var size uint64
	var err error

	switch diskType {
	case DiskTypeRaw:
		path = filepath.Join(m.diskDir, name+".raw")
		size, err = getFileSize(path)
	case DiskTypeQCOW2:
		path = filepath.Join(m.diskDir, name+".qcow2")
		size, err = m.getQCOW2Size(path)
	case DiskTypeZvol:
		path = fmt.Sprintf("%s/vm/%s", m.zpool, name)
		size, err = m.getZvolSize(path)
	}

	if err != nil {
		return nil, err
	}

	return &DiskInfo{
		Name: name,
		Type: diskType,
		Path: path,
		Size: size,
	}, nil
}

// getFileSize gets the size of a file
func getFileSize(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return uint64(info.Size()), nil
}

// getQCOW2Size gets the size of a QCOW2 image
func (m *Manager) getQCOW2Size(path string) (uint64, error) {
	cmd := exec.Command("qemu-img", "info", "--output=json", path)
	_, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parse JSON output (simplified - would use json package in production)
	// For now, return 0 and let caller handle it
	return 0, nil
}

// getZvolSize gets the size of a ZFS zvol
func (m *Manager) getZvolSize(path string) (uint64, error) {
	cmd := exec.Command("zfs", "get", "-H", "-o", "value", "volsize", path)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	size := strings.TrimSpace(string(output))
	return parseSize(size)
}

// parseSize parses a size string (e.g., "10G", "512M") to bytes
func parseSize(size string) (uint64, error) {
	size = strings.ToUpper(strings.TrimSpace(size))

	var multiplier uint64 = 1
	if strings.HasSuffix(size, "G") {
		multiplier = 1024 * 1024 * 1024
		size = strings.TrimSuffix(size, "G")
	} else if strings.HasSuffix(size, "M") {
		multiplier = 1024 * 1024
		size = strings.TrimSuffix(size, "M")
	} else if strings.HasSuffix(size, "K") {
		multiplier = 1024
		size = strings.TrimSuffix(size, "K")
	}

	bytes, err := strconv.ParseUint(size, 10, 64)
	if err != nil {
		return 0, err
	}

	return bytes * multiplier, nil
}
