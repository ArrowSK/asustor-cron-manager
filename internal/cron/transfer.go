package cron

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const ExportFormat = "asustor-cron-manager"
const ExportFormatVersion = 1

type PortableJob struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Enabled  bool   `json:"enabled"`
}

type ExportBundle struct {
	Format              string        `json:"format"`
	FormatVersion       int           `json:"formatVersion"`
	AppVersion          string        `json:"appVersion"`
	ExportedAt          time.Time     `json:"exportedAt"`
	SourceCrontabSHA256 string        `json:"sourceCrontabSha256"`
	Jobs                []PortableJob `json:"jobs"`
}

type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Replaced int `json:"replaced"`
}

func (s *Store) ExportManaged(appVersion string) (ExportBundle, error) {
	content, err := s.Read()
	if err != nil {
		return ExportBundle{}, err
	}
	managed, _ := Parse(content)
	jobs := make([]PortableJob, 0, len(managed))
	for _, b := range managed {
		jobs = append(jobs, PortableJob{Name: b.Name, Schedule: b.Schedule, Command: b.Command, Enabled: b.Enabled})
	}
	sum := sha256.Sum256([]byte(content))
	return ExportBundle{
		Format:              ExportFormat,
		FormatVersion:       ExportFormatVersion,
		AppVersion:          appVersion,
		ExportedAt:          time.Now().UTC(),
		SourceCrontabSHA256: hex.EncodeToString(sum[:]),
		Jobs:                jobs,
	}, nil
}

func ValidateExportBundle(b ExportBundle) error {
	if b.Format != ExportFormat {
		return fmt.Errorf("unsupported export format")
	}
	if b.FormatVersion != ExportFormatVersion {
		return fmt.Errorf("unsupported export format version %d", b.FormatVersion)
	}
	if len(b.Jobs) > 500 {
		return fmt.Errorf("export contains too many jobs")
	}
	for i, j := range b.Jobs {
		if strings.TrimSpace(j.Name) == "" {
			return fmt.Errorf("job %d has no name", i+1)
		}
		if len(j.Name) > 120 {
			return fmt.Errorf("job %d name is too long", i+1)
		}
		if err := ValidateSchedule(strings.TrimSpace(j.Schedule)); err != nil {
			return fmt.Errorf("job %d: %w", i+1, err)
		}
		cmd := strings.TrimSpace(j.Command)
		if cmd == "" || strings.ContainsAny(cmd, "\r\n\x00") {
			return fmt.Errorf("job %d has an invalid command", i+1)
		}
	}
	return nil
}

// ImportManaged imports portable Cron Manager jobs while preserving all raw/system
// crontab lines. mode may be "merge" or "replace-managed".
func (s *Store) ImportManaged(bundle ExportBundle, mode string) (ImportResult, error) {
	if err := ValidateExportBundle(bundle); err != nil {
		return ImportResult{}, err
	}
	if mode != "merge" && mode != "replace-managed" {
		return ImportResult{}, fmt.Errorf("invalid import mode")
	}

	content, err := s.Read()
	if err != nil {
		return ImportResult{}, err
	}
	managed, _ := Parse(content)
	result := ImportResult{}

	if mode == "replace-managed" && len(managed) > 0 {
		result.Replaced = len(managed)
		for i := len(managed) - 1; i >= 0; i-- {
			b := managed[i]
			content = ReplaceRange(content, b.Start, b.End, nil)
		}
		managed = nil
	}

	existing := make(map[string]struct{}, len(managed))
	for _, b := range managed {
		existing[b.Schedule+"\x00"+b.Command] = struct{}{}
	}

	for _, j := range bundle.Jobs {
		schedule := strings.TrimSpace(j.Schedule)
		command := strings.TrimSpace(j.Command)
		key := schedule + "\x00" + command
		if _, ok := existing[key]; ok {
			result.Skipped++
			continue
		}
		block := RenderManaged(ManagedBlock{
			ID: NewID(), Name: strings.TrimSpace(j.Name), Schedule: schedule,
			Command: command, Enabled: j.Enabled,
		}, s.RunnerPath)
		content = AppendBlock(content, block)
		existing[key] = struct{}{}
		result.Imported++
	}

	if result.Imported == 0 && result.Replaced == 0 {
		return result, nil
	}
	if err := s.Write(content, "import-jobs"); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}
