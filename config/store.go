package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const fileVersion = 3

// Store persists tunnel profiles as JSON under the user config directory.
type Store struct {
	path string
	mu   sync.Mutex
}

func DefaultPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SWITCHBOARD_CONFIG")); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Application Support", "Switchboard")
	return filepath.Join(dir, "tunnels.json"), nil
}

func Open(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := s.Save(Document{Version: fileVersion, Profiles: []Profile{}}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (Document, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Document{}, fmt.Errorf("parse config: %w", err)
	}
	if doc.Profiles == nil {
		doc.Profiles = []Profile{}
	}
	if doc.Version == 0 {
		doc.Version = fileVersion
	}
	// v1→v2: notifications default on for profiles that never persisted the flag.
	if doc.Version < 2 {
		for i := range doc.Profiles {
			doc.Profiles[i].NotifyOnStatusChange = true
		}
		doc.Version = 2
	}
	if doc.Preferences == nil {
		prefs := DefaultPreferences()
		doc.Preferences = &prefs
	} else {
		normalized := doc.Preferences.Normalize()
		doc.Preferences = &normalized
	}
	if doc.Version < fileVersion {
		doc.Version = fileVersion
		_ = s.saveLocked(doc)
	}
	return doc, nil
}

func (s *Store) Save(doc Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(doc)
}

func (s *Store) saveLocked(doc Document) error {
	doc.Version = fileVersion
	if doc.Profiles == nil {
		doc.Profiles = []Profile{}
	}
	if doc.Preferences == nil {
		prefs := DefaultPreferences()
		doc.Preferences = &prefs
	} else {
		normalized := doc.Preferences.Normalize()
		doc.Preferences = &normalized
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Upsert(p Profile) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return Document{}, err
	}
	found := false
	for i := range doc.Profiles {
		if doc.Profiles[i].ID == p.ID {
			doc.Profiles[i] = p
			found = true
			break
		}
	}
	if !found {
		doc.Profiles = append(doc.Profiles, p)
	}
	if err := s.saveLocked(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (s *Store) Delete(id string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return Document{}, err
	}
	out := doc.Profiles[:0]
	for _, p := range doc.Profiles {
		if p.ID != id {
			out = append(out, p)
		}
	}
	doc.Profiles = out
	if err := s.saveLocked(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (s *Store) Get(id string) (Profile, bool, error) {
	doc, err := s.Load()
	if err != nil {
		return Profile{}, false, err
	}
	for _, p := range doc.Profiles {
		if p.ID == id {
			return p, true, nil
		}
	}
	return Profile{}, false, nil
}

// Preferences returns app-wide settings.
func (s *Store) Preferences() (Preferences, error) {
	doc, err := s.Load()
	if err != nil {
		return DefaultPreferences(), err
	}
	if doc.Preferences == nil {
		return DefaultPreferences(), nil
	}
	return doc.Preferences.Normalize(), nil
}

// SetPreferences persists app-wide settings.
func (s *Store) SetPreferences(prefs Preferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return err
	}
	normalized := prefs.Normalize()
	doc.Preferences = &normalized
	return s.saveLocked(doc)
}
