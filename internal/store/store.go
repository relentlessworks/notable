package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/relentlessworks/notable/internal/models"
)

// Store is a file-backed persistent store using JSON.
// It uses a simple but robust approach: load all data into memory at startup,
// persist changes atomically (write to temp file, rename).
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *storeData
}

type storeData struct {
	Workspaces map[string]*models.Workspace `json:"workspaces"`
	Notes      map[string]*models.Note      `json:"notes"`
	Tokens     map[string]*models.Token     `json:"tokens"`
	OTPs       map[string]*otpEntry         `json:"otps"`
}

type otpEntry struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	Workspace string    `json:"workspace"`
	ExpiresAt time.Time `json:"expires_at"`
}

// New opens (or creates) the store file.
func New(path string) (*Store, error) {
	s := &Store{
		filePath: path,
		data: &storeData{
			Workspaces: make(map[string]*models.Workspace),
			Notes:      make(map[string]*models.Note),
			Tokens:     make(map[string]*models.Token),
			OTPs:       make(map[string]*otpEntry),
		},
	}

	// Load existing data if file exists
	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("load store: %w", err)
		}
	}

	return s, nil
}

// Close persists any pending changes.
func (s *Store) Close() error {
	return s.save()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	// Empty file = fresh store
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, s.data)
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	dir := filepath.Dir(s.filePath)
	if dir == "" {
		dir = "."
	}
	tmpFile, err := os.CreateTemp(dir, ".notable-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(b); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}

// --- Workspace operations ---

func (s *Store) CreateWorkspace(handle, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Workspaces[handle] = &models.Workspace{
		Handle: handle,
		Name:   name,
		Plan:   "free",
	}
	return s.save()
}

func (s *Store) GetWorkspace(handle string) (*models.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.data.Workspaces[handle]
	if !ok {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

func (s *Store) WorkspaceExists(handle string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data.Workspaces[handle]
	return ok, nil
}

// --- Note operations ---

func (s *Store) CreateNote(handle, title, body string, tags []string, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.data.Notes[handle] = &models.Note{
		Handle:    handle,
		Title:     title,
		Body:      body,
		Tags:      tags,
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.save()
}

func (s *Store) GetNote(handle string) (*models.Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	note, ok := s.data.Notes[handle]
	if !ok {
		return nil, fmt.Errorf("note not found")
	}
	return note, nil
}

func (s *Store) ListNotes(workspace string, tag string, limit int) ([]*models.Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var notes []*models.Note
	for _, n := range s.data.Notes {
		if n.Workspace != workspace {
			continue
		}
		if tag != "" {
			found := false
			for _, t := range n.Tags {
				if strings.EqualFold(t, tag) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		notes = append(notes, n)
	}

	// Sort by created_at descending
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	if len(notes) > limit {
		notes = notes[:limit]
	}
	return notes, nil
}

func (s *Store) UpdateNote(handle, title, body string, tags []string, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, ok := s.data.Notes[handle]
	if !ok || note.Workspace != workspace {
		return fmt.Errorf("note not found")
	}
	if title != "" {
		note.Title = title
	}
	if body != "" {
		note.Body = body
	}
	if tags != nil {
		note.Tags = tags
	}
	note.UpdatedAt = time.Now()
	return s.save()
}

func (s *Store) DeleteNote(handle, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, ok := s.data.Notes[handle]
	if !ok || note.Workspace != workspace {
		return fmt.Errorf("note not found")
	}
	delete(s.data.Notes, handle)
	return s.save()
}

func (s *Store) SearchNotes(workspace, query string, limit int) ([]*models.Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	queryLower := strings.ToLower(query)
	var notes []*models.Note
	for _, n := range s.data.Notes {
		if n.Workspace != workspace {
			continue
		}
		if strings.Contains(strings.ToLower(n.Title), queryLower) ||
			strings.Contains(strings.ToLower(n.Body), queryLower) {
			notes = append(notes, n)
		}
	}

	// Sort by created_at descending
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	if len(notes) > limit {
		notes = notes[:limit]
	}
	return notes, nil
}

// --- Token operations ---

func (s *Store) CreateToken(value, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Tokens[value] = &models.Token{
		Value:     value,
		Workspace: workspace,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetToken(value string) (*models.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.data.Tokens[value]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	return token, nil
}

func (s *Store) DeleteToken(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Tokens, value)
	return s.save()
}

// --- OTP operations ---

func (s *Store) SaveOTP(email, code, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	s.data.OTPs[key] = &otpEntry{
		Email:     email,
		Code:      code,
		Workspace: workspace,
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetOTP(email, code string) (string, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := email + ":" + code
	entry, ok := s.data.OTPs[key]
	if !ok {
		return "", time.Time{}, fmt.Errorf("OTP not found")
	}
	return entry.Workspace, entry.ExpiresAt, nil
}

func (s *Store) DeleteOTP(email, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	delete(s.data.OTPs, key)
	return s.save()
}
