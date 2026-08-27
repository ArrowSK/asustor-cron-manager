package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	latestReleaseAPI  = "https://api.github.com/repos/ArrowSK/asustor-cron-manager/releases/latest"
	binaryAssetName   = "cron-manager_linux_arm64"
	checksumAssetName = "cron-manager_linux_arm64.sha256"
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

type Status struct {
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion,omitempty"`
	UpdateAvailable bool      `json:"updateAvailable"`
	ReleaseURL      string    `json:"releaseUrl,omitempty"`
	ReleaseNotes    string    `json:"releaseNotes,omitempty"`
	PublishedAt     time.Time `json:"publishedAt,omitempty"`
	CanAutoUpdate   bool      `json:"canAutoUpdate"`
}

type Manager struct {
	CurrentVersion string
	BinaryPath     string
	HTTP           *http.Client
}

func New(currentVersion, binaryPath string) *Manager {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{
		Timeout:   45 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" || !trustedHost(req.URL.Hostname()) {
				return fmt.Errorf("refusing untrusted update redirect to %s", req.URL.Host)
			}
			return nil
		},
	}
	return &Manager{CurrentVersion: currentVersion, BinaryPath: binaryPath, HTTP: client}
}

func trustedHost(host string) bool {
	host = strings.ToLower(host)
	return host == "github.com" || host == "api.github.com" ||
		strings.HasSuffix(host, ".githubusercontent.com") ||
		strings.HasSuffix(host, ".github.com")
}

func (m *Manager) Check(ctx context.Context) (Status, error) {
	rel, err := m.release(ctx)
	if err != nil {
		return Status{}, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if latest == "" {
		return Status{}, fmt.Errorf("latest GitHub release has no version tag")
	}
	_, hasBinary := findAsset(rel.Assets, binaryAssetName)
	_, hasChecksum := findAsset(rel.Assets, checksumAssetName)
	return Status{
		CurrentVersion:  m.CurrentVersion,
		LatestVersion:   latest,
		UpdateAvailable: compareVersions(latest, m.CurrentVersion) > 0,
		ReleaseURL:      rel.HTMLURL,
		ReleaseNotes:    truncate(strings.TrimSpace(rel.Body), 4000),
		PublishedAt:     rel.PublishedAt,
		CanAutoUpdate:   hasBinary && hasChecksum,
	}, nil
}

func (m *Manager) Apply(ctx context.Context) (Status, error) {
	rel, err := m.release(ctx)
	if err != nil {
		return Status{}, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if compareVersions(latest, m.CurrentVersion) <= 0 {
		return Status{CurrentVersion: m.CurrentVersion, LatestVersion: latest, UpdateAvailable: false, ReleaseURL: rel.HTMLURL, CanAutoUpdate: true}, fmt.Errorf("Cron Manager is already up to date")
	}
	binAsset, ok := findAsset(rel.Assets, binaryAssetName)
	if !ok {
		return Status{}, fmt.Errorf("release %s has no %s asset", latest, binaryAssetName)
	}
	sumAsset, ok := findAsset(rel.Assets, checksumAssetName)
	if !ok {
		return Status{}, fmt.Errorf("release %s has no checksum asset", latest)
	}

	expectedText, err := m.downloadText(ctx, sumAsset.URL, 64*1024)
	if err != nil {
		return Status{}, fmt.Errorf("download checksum: %w", err)
	}
	expected, err := parseChecksum(expectedText)
	if err != nil {
		return Status{}, err
	}

	if err := os.MkdirAll(filepath.Dir(m.BinaryPath), 0o755); err != nil {
		return Status{}, err
	}
	staged := m.BinaryPath + ".update"
	_ = os.Remove(staged)
	if err := m.downloadFile(ctx, binAsset.URL, staged, 64*1024*1024); err != nil {
		return Status{}, fmt.Errorf("download update: %w", err)
	}
	defer os.Remove(staged)

	actual, err := fileSHA256(staged)
	if err != nil {
		return Status{}, err
	}
	if !strings.EqualFold(actual, expected) {
		return Status{}, fmt.Errorf("update checksum mismatch")
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return Status{}, err
	}

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(checkCtx, staged, "version").CombinedOutput()
	if err != nil {
		return Status{}, fmt.Errorf("downloaded binary validation failed: %w", err)
	}
	reported := strings.TrimSpace(string(out))
	if reported != latest {
		return Status{}, fmt.Errorf("downloaded binary reports version %q, expected %q", reported, latest)
	}

	previous := m.BinaryPath + ".previous"
	_ = os.Remove(previous)
	if err := os.Rename(m.BinaryPath, previous); err != nil {
		return Status{}, fmt.Errorf("preserve current binary: %w", err)
	}
	if err := os.Rename(staged, m.BinaryPath); err != nil {
		_ = os.Rename(previous, m.BinaryPath)
		return Status{}, fmt.Errorf("activate update: %w", err)
	}

	return Status{
		CurrentVersion:  m.CurrentVersion,
		LatestVersion:   latest,
		UpdateAvailable: true,
		ReleaseURL:      rel.HTMLURL,
		ReleaseNotes:    truncate(strings.TrimSpace(rel.Body), 4000),
		PublishedAt:     rel.PublishedAt,
		CanAutoUpdate:   true,
	}, nil
}

// RestartIntoUpdatedBinary replaces this process image after delay. PID is
// preserved, so ADM's existing pidfile remains correct. If exec fails, the
// previous binary is restored and the current process keeps serving.
func (m *Manager) RestartIntoUpdatedBinary(delay time.Duration, logger func(string, ...any)) {
	go func() {
		time.Sleep(delay)
		err := syscall.Exec(m.BinaryPath, []string{m.BinaryPath}, os.Environ())
		if err == nil {
			return
		}
		previous := m.BinaryPath + ".previous"
		broken := m.BinaryPath + ".failed"
		_ = os.Remove(broken)
		_ = os.Rename(m.BinaryPath, broken)
		_ = os.Rename(previous, m.BinaryPath)
		if logger != nil {
			logger("self-update restart failed and binary was rolled back: %v", err)
		}
	}()
}

func (m *Manager) release(ctx context.Context) (githubRelease, error) {
	var rel githubRelease
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return rel, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "asustor-cron-manager/"+m.CurrentVersion)
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return rel, fmt.Errorf("GitHub release check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return rel, fmt.Errorf("no published Cron Manager release found on GitHub")
	}
	if resp.StatusCode != http.StatusOK {
		return rel, fmt.Errorf("GitHub release check returned HTTP %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024))
	if err := dec.Decode(&rel); err != nil {
		return rel, fmt.Errorf("decode GitHub release: %w", err)
	}
	return rel, nil
}

func (m *Manager) downloadText(ctx context.Context, rawURL string, limit int64) (string, error) {
	req, err := m.downloadRequest(ctx, rawURL)
	if err != nil {
		return "", err
	}
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(b)) > limit {
		return "", fmt.Errorf("download exceeds size limit")
	}
	return string(b), nil
}

func (m *Manager) downloadFile(ctx context.Context, rawURL, path string, limit int64) error {
	req, err := m.downloadRequest(ctx, rawURL)
	if err != nil {
		return err
	}
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("download exceeds size limit")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit {
		return fmt.Errorf("download exceeds size limit")
	}
	return nil
}

func (m *Manager) downloadRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || !trustedHost(u.Hostname()) {
		return nil, fmt.Errorf("untrusted release asset URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "asustor-cron-manager/"+m.CurrentVersion)
	return req, nil
}

func findAsset(assets []Asset, name string) (Asset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

func parseChecksum(s string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		v := strings.ToLower(fields[0])
		if len(v) != 64 {
			continue
		}
		if _, err := hex.DecodeString(v); err == nil {
			return v, nil
		}
	}
	return "", fmt.Errorf("invalid update checksum file")
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func compareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		av, bv := 0, 0
		if i < len(pa) {
			av = pa[i]
		}
		if i < len(pb) {
			bv = pb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
