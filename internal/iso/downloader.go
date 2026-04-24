package iso

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Downloader handles ISO downloads
type Downloader struct {
	isoDir    string
	tempDir   string
	maxSize   int64
	timeout   time.Duration
}

// DownloadResult holds the result of a download operation
type DownloadResult struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Duration string `json:"duration"`
}

// NewDownloader creates a new ISO downloader
func NewDownloader(isoDir string, maxSize string) (*Downloader, error) {
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create ISO directory: %w", err)
	}

	maxSizeBytes, err := parseSize(maxSize)
	if err != nil {
		maxSizeBytes = 10 * 1024 * 1024 * 1024 // default 10GB
	}

	return &Downloader{
		isoDir:  isoDir,
		tempDir: filepath.Join(isoDir, ".tmp"),
		maxSize: maxSizeBytes,
		timeout: 30 * time.Minute,
	}, nil
}

// Download downloads an ISO from a URL
func (d *Downloader) Download(url, filename, expectedSHA256 string) (*DownloadResult, error) {
	if err := os.MkdirAll(d.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	tempPath := filepath.Join(d.tempDir, filename+".part")
	finalPath := filepath.Join(d.isoDir, filename)

	// Check if file already exists
	if _, err := os.Stat(finalPath); err == nil {
		return nil, fmt.Errorf("ISO already exists: %s", filename)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: d.timeout,
	}

	// Start download
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Check content length
	if resp.ContentLength > d.maxSize {
		return nil, fmt.Errorf("ISO size exceeds maximum allowed (%d > %d)", resp.ContentLength, d.maxSize)
	}

	// Create temp file
	out, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Create hash writer
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	// Copy with progress tracking
	startTime := time.Now()
	written, err := io.Copy(writer, &readCloser{Reader: resp.Body})
	if err != nil {
		out.Close()
		os.Remove(tempPath)
		return nil, fmt.Errorf("download failed: %w", err)
	}

	out.Close()

	// Calculate actual SHA256
	actualSHA256 := fmt.Sprintf("%x", hash.Sum(nil))

	// Verify checksum if provided
	if expectedSHA256 != "" {
		if !strings.EqualFold(actualSHA256, expectedSHA256) {
			os.Remove(tempPath)
			return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
		}
	}

	// Move to final location
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to move ISO to final location: %w", err)
	}

	duration := time.Since(startTime)

	return &DownloadResult{
		Name:     filename,
		Path:     finalPath,
		Size:     written,
		SHA256:   actualSHA256,
		Duration: duration.String(),
	}, nil
}

// Verify verifies the checksum of an existing ISO
func (d *Downloader) Verify(filename, expectedSHA256 string) (bool, string, error) {
	path := filepath.Join(d.isoDir, filename)

	file, err := os.Open(path)
	if err != nil {
		return false, "", fmt.Errorf("failed to open ISO: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualSHA256 := fmt.Sprintf("%x", hash.Sum(nil))

	if expectedSHA256 != "" && !strings.EqualFold(actualSHA256, expectedSHA256) {
		return false, actualSHA256, fmt.Errorf("checksum mismatch")
	}

	return true, actualSHA256, nil
}

// Delete deletes an ISO file
func (d *Downloader) Delete(filename string) error {
	path := filepath.Join(d.isoDir, filename)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ISO not found: %s", filename)
		}
		return fmt.Errorf("failed to delete ISO: %w", err)
	}

	return nil
}

// List lists all ISO files in the directory
func (d *Downloader) List() ([]string, error) {
	var isos []string

	entries, err := os.ReadDir(d.isoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read ISO directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip temp files
		if strings.HasSuffix(name, ".part") {
			continue
		}
		isos = append(isos, name)
	}

	return isos, nil
}

// GetInfo gets information about an ISO file
func (d *Downloader) GetInfo(filename string) (map[string]interface{}, error) {
	path := filepath.Join(d.isoDir, filename)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat ISO: %w", err)
	}

	// Calculate SHA256
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":      filename,
		"path":      path,
		"size":      info.Size(),
		"sha256":    fmt.Sprintf("%x", hash.Sum(nil)),
		"modified":  info.ModTime(),
	}, nil
}

// readCloser wraps an io.Reader to implement ReadCloser
type readCloser struct {
	io.Reader
}

func (rc *readCloser) Close() error {
	return nil
}

// parseSize parses a size string to bytes
func parseSize(size string) (int64, error) {
	size = strings.ToUpper(strings.TrimSpace(size))

	multiplier := int64(1)
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

	val, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, err
	}

	return val * multiplier, nil
}
