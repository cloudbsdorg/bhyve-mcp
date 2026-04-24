package console

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ConsoleManager manages VM console access
type ConsoleManager struct {
	logDir      string
	maxLogSize  int64
	maxLogFiles int
	consoles    map[string]*ConsoleSession
	mu          sync.RWMutex
}

// ConsoleSession represents an active console session
type ConsoleSession struct {
	VMName   string
	Device   string
	File     *os.File
	Reader   *bufio.Reader
	mu       sync.Mutex
	lastRead time.Time
}

// NewConsoleManager creates a new console manager
func NewConsoleManager(logDir string, maxLogSize string, maxLogFiles int) (*ConsoleManager, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create console log directory: %w", err)
	}

	maxSize, err := parseSize(maxLogSize)
	if err != nil {
		maxSize = 100 * 1024 * 1024 // default 100MB
	}

	return &ConsoleManager{
		logDir:      logDir,
		maxLogSize:  maxSize,
		maxLogFiles: maxLogFiles,
		consoles:    make(map[string]*ConsoleSession),
	}, nil
}

// Open opens a console session for a VM
func (m *ConsoleManager) Open(vmName, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.consoles[vmName]; exists {
		return fmt.Errorf("console already open for VM: %s", vmName)
	}

	// Open the console device
	file, err := os.OpenFile(device, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open console device: %w", err)
	}

	session := &ConsoleSession{
		VMName:   vmName,
		Device:   device,
		File:     file,
		Reader:   bufio.NewReader(file),
		lastRead: time.Now(),
	}

	m.consoles[vmName] = session
	return nil
}

// Close closes a console session
func (m *ConsoleManager) Close(vmName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.consoles[vmName]
	if !exists {
		return fmt.Errorf("no console session for VM: %s", vmName)
	}

	if err := session.File.Close(); err != nil {
		return fmt.Errorf("failed to close console: %w", err)
	}

	delete(m.consoles, vmName)
	return nil
}

// Read reads from the console
func (m *ConsoleManager) Read(vmName string, timeout time.Duration) (string, error) {
	m.mu.RLock()
	session, exists := m.consoles[vmName]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("no console session for VM: %s", vmName)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Set read deadline
	session.File.SetReadDeadline(time.Now().Add(timeout))

	// Read available data
	buf := make([]byte, 4096)
	n, err := session.Reader.Read(buf)
	if err != nil {
		if err == io.EOF {
			return string(buf[:n]), nil
		}
		// Check if it's a timeout error
		if opErr, ok := err.(*os.PathError); ok {
			if opErr.Timeout() {
				return "", nil // No data available
			}
		}
		return "", fmt.Errorf("failed to read from console: %w", err)
	}

	session.lastRead = time.Now()
	return string(buf[:n]), nil
}

// Write writes to the console
func (m *ConsoleManager) Write(vmName string, data string) error {
	m.mu.RLock()
	session, exists := m.consoles[vmName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no console session for VM: %s", vmName)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	_, err := session.File.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("failed to write to console: %w", err)
	}

	return nil
}

// SendKeys sends keystrokes to the console
func (m *ConsoleManager) SendKeys(vmName string, keys string) error {
	return m.Write(vmName, keys)
}

// SendText sends text with proper line endings to the console
func (m *ConsoleManager) SendText(vmName string, text string) error {
	// Convert to Unix line endings
	text = text + "\n"
	return m.Write(vmName, text)
}

// GetLogPath returns the path to a VM's console log
func (m *ConsoleManager) GetLogPath(vmName string) string {
	return filepath.Join(m.logDir, fmt.Sprintf("%s.log", vmName))
}

// ReadLog reads the console log for a VM
func (m *ConsoleManager) ReadLog(vmName string, lines int) ([]string, error) {
	logPath := m.GetLogPath(vmName)

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read last N lines
	var result []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result = append(result, scanner.Text())
		if len(result) > lines {
			result = result[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	return result, nil
}

// PersistLog persists console output to a log file
func (m *ConsoleManager) PersistLog(vmName string, data string) error {
	logPath := m.GetLogPath(vmName)

	// Check log file size and rotate if needed
	if info, err := os.Stat(logPath); err == nil {
		if info.Size() > m.maxLogSize {
			if err := m.rotateLog(vmName); err != nil {
				return fmt.Errorf("failed to rotate log: %w", err)
			}
		}
	}

	// Append to log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Add timestamp
	timestamp := time.Now().Format(time.RFC3339)
	_, err = fmt.Fprintf(file, "[%s] %s", timestamp, data)
	return err
}

// rotateLog rotates the console log file
func (m *ConsoleManager) rotateLog(vmName string) error {
	basePath := m.GetLogPath(vmName)

	// Remove oldest log if at limit
	oldestPath := fmt.Sprintf("%s.%d", basePath, m.maxLogFiles-1)
	os.Remove(oldestPath)

	// Rotate existing logs
	for i := m.maxLogFiles - 2; i >= 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", basePath, i)
		newPath := fmt.Sprintf("%s.%d", basePath, i+1)
		os.Rename(oldPath, newPath)
	}

	// Move current log to .0
	if _, err := os.Stat(basePath); err == nil {
		if err := os.Rename(basePath, basePath+".0"); err != nil {
			return err
		}
	}

	return nil
}

// ListSessions lists active console sessions
func (m *ConsoleManager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]string, 0, len(m.consoles))
	for vmName := range m.consoles {
		sessions = append(sessions, vmName)
	}
	return sessions
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
