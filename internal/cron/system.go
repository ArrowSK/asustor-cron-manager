package cron

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	BackupDir  string
	RunnerPath string
	mu         sync.Mutex
}

func NewStore(backupDir, runnerPath string) *Store {
	return &Store{BackupDir: backupDir, RunnerPath: runnerPath}
}

func (s *Store) Read() (string, error) {
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := string(out)
		if strings.Contains(strings.ToLower(text), "no crontab") {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l: %w: %s", err, strings.TrimSpace(text))
	}
	return string(out), nil
}

func (s *Store) Write(content, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.Read()
	if err != nil {
		return err
	}
	if current == content {
		return nil
	}
	if _, err := s.backupUnlocked(current, reason); err != nil {
		return err
	}
	cmd := exec.Command("crontab")
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install crontab: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (s *Store) backupUnlocked(content, reason string) (string, error) {
	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		return "", err
	}
	reason = sanitize(reason)
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + reason + ".cron"
	path := filepath.Join(s.BackupDir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	_ = s.pruneBackups(50)
	return name, nil
}

func sanitize(s string) string {
	if s == "" {
		return "change"
	}
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *Store) pruneBackups(keep int) error {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".cron") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) <= keep {
		return nil
	}
	for _, n := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(s.BackupDir, n))
	}
	return nil
}

func (s *Store) Backups() ([]string, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".cron") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (s *Store) Restore(name string) error {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".cron") {
		return fmt.Errorf("invalid backup name")
	}
	b, err := os.ReadFile(filepath.Join(s.BackupDir, name))
	if err != nil {
		return err
	}
	return s.Write(string(b), "pre-restore")
}
