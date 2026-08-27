package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type config struct {
	Salt       string `json:"salt"`
	Hash       string `json:"hash"`
	Iterations int    `json:"iterations"`
}

type Manager struct {
	path     string
	mu       sync.Mutex
	sessions map[string]time.Time
}

func New(path string) *Manager { return &Manager{path: path, sessions: map[string]time.Time{}} }

func (m *Manager) SetupRequired() bool {
	_, err := os.Stat(m.path)
	return os.IsNotExist(err)
}

func (m *Manager) Setup(password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(password) < 10 {
		return fmt.Errorf("password must be at least 10 characters")
	}
	if _, err := os.Stat(m.path); err == nil {
		return fmt.Errorf("authentication is already configured")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	const iterations = 180000
	hash := pbkdf2SHA256([]byte(password), salt, iterations, 32)
	cfg := config{Salt: base64.RawStdEncoding.EncodeToString(salt), Hash: base64.RawStdEncoding.EncodeToString(hash), Iterations: iterations}
	return writeConfig(m.path, cfg)
}

func (m *Manager) Verify(password string) bool {
	cfg, err := readConfig(m.path)
	if err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(cfg.Salt)
	expected, err2 := base64.RawStdEncoding.DecodeString(cfg.Hash)
	if err1 != nil || err2 != nil || cfg.Iterations < 10000 {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, cfg.Iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (m *Manager) Change(oldPassword, newPassword string) error {
	if !m.Verify(oldPassword) {
		return fmt.Errorf("current password is incorrect")
	}
	if len(newPassword) < 10 {
		return fmt.Errorf("new password must be at least 10 characters")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	const iterations = 180000
	hash := pbkdf2SHA256([]byte(newPassword), salt, iterations, 32)
	if err := writeConfig(m.path, config{Salt: base64.RawStdEncoding.EncodeToString(salt), Hash: base64.RawStdEncoding.EncodeToString(hash), Iterations: iterations}); err != nil {
		return err
	}
	m.sessions = map[string]time.Time{}
	return nil
}

func (m *Manager) NewSession() (string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().Add(12 * time.Hour)
	m.sessions[token] = expires
	return token, expires, nil
}

func (m *Manager) ValidSession(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(m.sessions, token)
		return false
	}
	return true
}

func (m *Manager) Revoke(token string) { m.mu.Lock(); delete(m.sessions, token); m.mu.Unlock() }

func readConfig(path string) (config, error) {
	var cfg config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

func writeConfig(path string, cfg config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := 32
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
