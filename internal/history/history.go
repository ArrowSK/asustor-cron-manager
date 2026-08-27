package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Record struct {
	RunID    string    `json:"runId"`
	JobID    string    `json:"jobId"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	ExitCode int       `json:"exitCode"`
	Source   string    `json:"source"`
	Output   string    `json:"output,omitempty"`
}

type Store struct{ Path, LockPath string }

func New(path string) *Store { return &Store{Path: path, LockPath: path + ".lock"} }

func (s *Store) Add(r Record) error {
	return s.withLock(func() error {
		all, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		all[r.JobID] = append([]Record{r}, all[r.JobID]...)
		if len(all[r.JobID]) > 20 {
			all[r.JobID] = all[r.JobID][:20]
		}
		return s.saveUnlocked(all)
	})
}

func (s *Store) Latest(jobID string) (*Record, error) {
	var result *Record
	err := s.withLock(func() error {
		all, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if rows := all[jobID]; len(rows) > 0 {
			r := rows[0]
			result = &r
		}
		return nil
	})
	return result, err
}

func (s *Store) List(jobID string) ([]Record, error) {
	var rows []Record
	err := s.withLock(func() error {
		all, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		rows = append(rows, all[jobID]...)
		return nil
	})
	return rows, err
}

func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.LockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

func (s *Store) loadUnlocked() (map[string][]Record, error) {
	all := map[string][]Record{}
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return all, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return all, nil
	}
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	return all, nil
}

func (s *Store) saveUnlocked(all map[string][]Record) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
