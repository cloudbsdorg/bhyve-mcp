package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store provides persistent state storage
type Store struct {
	path string
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewStore creates a new state store
func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	store := &Store{
		path: filepath.Join(path, "state.json"),
		data: make(map[string]interface{}),
	}

	// Load existing state if available
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return store, nil
}

// load reads state from disk
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.data)
}

// save writes state to disk
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Get retrieves a value from the store
func (s *Store) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	return val, exists
}

// GetString retrieves a string value from the store
func (s *Store) GetString(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		return "", false
	}

	str, ok := val.(string)
	return str, ok
}

// Set stores a value
func (s *Store) Set(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value

	return s.save()
}

// Delete removes a value from the store
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)

	return s.save()
}

// GetAll returns all stored data
func (s *Store) GetAll() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// ISORecord represents metadata about an ISO file
type ISORecord struct {
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size"`
	Downloaded string `json:"downloaded"`
	Verified   bool   `json:"verified"`
}

// ISOStore manages ISO files and metadata
type ISOStore struct {
	store   *Store
	isoDir  string
	dbPath  string
	mu      sync.RWMutex
	isos    map[string]*ISORecord
}

// NewISOStore creates a new ISO store
func NewISOStore(store *Store, isoDir string) (*ISOStore, error) {
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create ISO directory: %w", err)
	}

	is := &ISOStore{
		store:  store,
		isoDir: isoDir,
		dbPath: "isos",
		isos:   make(map[string]*ISORecord),
	}

	// Load ISO database
	if err := is.load(); err != nil {
		// Initialize empty database
		is.isos = make(map[string]*ISORecord)
	}

	return is, nil
}

// load loads the ISO database
func (s *ISOStore) load() error {
	data, exists := s.store.Get(s.dbPath)
	if !exists {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var isos map[string]*ISORecord
	if err := json.Unmarshal(jsonData, &isos); err != nil {
		return err
	}

	s.isos = isos
	return nil
}

// save saves the ISO database
func (s *ISOStore) save() error {
	return s.store.Set(s.dbPath, s.isos)
}

// Add adds an ISO record
func (s *ISOStore) Add(record *ISORecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isos[record.Name] = record
	return s.save()
}

// Get retrieves an ISO record
func (s *ISOStore) Get(name string) (*ISORecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.isos[name]
	return record, exists
}

// List returns all ISO records
func (s *ISOStore) List() []*ISORecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]*ISORecord, 0, len(s.isos))
	for _, record := range s.isos {
		records = append(records, record)
	}
	return records
}

// Delete removes an ISO record
func (s *ISOStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.isos, name)
	return s.save()
}

// GetPath returns the full path to an ISO file
func (s *ISOStore) GetPath(name string) string {
	return filepath.Join(s.isoDir, name)
}

// TemplateRecord represents a golden master template
type TemplateRecord struct {
	Name      string `json:"name"`
	SourceVM  string `json:"source_vm"`
	Created   string `json:"created"`
	Size      int64  `json:"size"`
	DiskType  string `json:"disk_type"`
	DiskPath  string `json:"disk_path"`
}

// TemplateStore manages VM templates
type TemplateStore struct {
	store       *Store
	templateDir string
	dbPath      string
	mu          sync.RWMutex
	templates   map[string]*TemplateRecord
}

// NewTemplateStore creates a new template store
func NewTemplateStore(store *Store, templateDir string) (*TemplateStore, error) {
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create template directory: %w", err)
	}

	ts := &TemplateStore{
		store:       store,
		templateDir: templateDir,
		dbPath:      "templates",
		templates:   make(map[string]*TemplateRecord),
	}

	// Load template database
	if err := ts.load(); err != nil {
		// Initialize empty database
		ts.templates = make(map[string]*TemplateRecord)
	}

	return ts, nil
}

// load loads the template database
func (t *TemplateStore) load() error {
	data, exists := t.store.Get(t.dbPath)
	if !exists {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var templates map[string]*TemplateRecord
	if err := json.Unmarshal(jsonData, &templates); err != nil {
		return err
	}

	t.templates = templates
	return nil
}

// save saves the template database
func (t *TemplateStore) save() error {
	return t.store.Set(t.dbPath, t.templates)
}

// Add adds a template record
func (t *TemplateStore) Add(record *TemplateRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.templates[record.Name] = record
	return t.save()
}

// Get retrieves a template record
func (t *TemplateStore) Get(name string) (*TemplateRecord, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record, exists := t.templates[name]
	return record, exists
}

// List returns all template records
func (t *TemplateStore) List() []*TemplateRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records := make([]*TemplateRecord, 0, len(t.templates))
	for _, record := range t.templates {
		records = append(records, record)
	}
	return records
}

// Delete removes a template record
func (t *TemplateStore) Delete(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.templates, name)
	return t.save()
}

// GetPath returns the full path to a template file
func (t *TemplateStore) GetPath(name string) string {
	return filepath.Join(t.templateDir, name)
}
