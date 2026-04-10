package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const planFilename = "plan.md"

// Store reads and writes plan.md in the project directory.
// plan.md IS the source of truth — human-editable, git-visible, session-surviving.
type Store struct {
	mu  sync.RWMutex
	cwd string
}

func NewStore(cwd string) (*Store, error) {
	return &Store{cwd: cwd}, nil
}

func (s *Store) Save(p *Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	content := FormatMarkdown(p)
	tmp := s.planPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("plan: write: %w", err)
	}
	return os.Rename(tmp, s.planPath())
}

func (s *Store) Load(_ string) (*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadFile()
}

func (s *Store) Active() (*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadFile()
}

func (s *Store) List() ([]*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, err := s.loadFile()
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return []*Plan{p}, nil
}

func (s *Store) Delete(_ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.planPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("plan: delete: %w", err)
	}
	return nil
}

func (s *Store) SetActive(_ string) error {
	// With file-per-directory model, there's only one plan — always active
	return nil
}

func (s *Store) Deactivate() error {
	// No-op: plan.md stays on disk, just not injected into prompt
	// The plan is "deactivated" by deleting the file
	return s.Delete("")
}

func (s *Store) planPath() string {
	return filepath.Join(s.cwd, planFilename)
}

func (s *Store) loadFile() (*Plan, error) {
	data, err := os.ReadFile(s.planPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("plan: read: %w", err)
	}

	return ParseMarkdown(string(data))
}
