package cron

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

func NewID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Store) ListJobs() ([]Job, error) {
	content, err := s.Read()
	if err != nil {
		return nil, err
	}
	managed, raw := Parse(content)
	jobs := make([]Job, 0, len(managed)+len(raw))
	for _, b := range managed {
		jobs = append(jobs, Job{ID: b.ID, Name: b.Name, Schedule: b.Schedule, Command: b.Command, Enabled: b.Enabled, Managed: true, Human: Humanize(b.Schedule)})
	}
	for _, r := range raw {
		name := inferName(r.Command)
		jobs = append(jobs, Job{ID: r.ID, Name: name, Schedule: r.Schedule, Command: r.Command, Enabled: true, Managed: false, Human: Humanize(r.Schedule)})
	}
	return jobs, nil
}

func inferName(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "Imported job"
	}

	// Skip leading NAME=value assignments commonly used by ADM-generated cron
	// entries, so TAG=CERTIFICATE certificate ... is named after certificate.
	i := 0
	for i < len(fields) && isCommandAssignment(fields[i]) {
		i++
	}
	if i >= len(fields) {
		return "Imported job"
	}

	candidate := fields[i]
	base := pathBase(candidate)

	// docker exec <container> <program> ... is common for NAS maintenance.
	// Prefer the program/script inside the container over the generic "docker".
	if base == "docker" && i+3 < len(fields) && fields[i+1] == "exec" {
		k := i + 3
		inner := pathBase(fields[k])
		if (inner == "sh" || inner == "bash" || strings.HasPrefix(inner, "python")) && k+1 < len(fields) {
			k++
		}
		candidate = fields[k]
		base = pathBase(candidate)
	}

	// If cron invokes a shell/interpreter explicitly, the next path is usually
	// a much more useful name than "sh" or "python3".
	if (base == "sh" || base == "bash" || strings.HasPrefix(base, "python")) && i+1 < len(fields) {
		candidate = fields[i+1]
		base = pathBase(candidate)
	}
	if base == "" {
		base = "job"
	}
	return "Imported: " + base
}

func isCommandAssignment(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range s[:eq] {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func pathBase(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func (s *Store) SaveJob(id, name, schedule, command string, enabled bool) (string, error) {
	name = strings.TrimSpace(name)
	schedule = strings.TrimSpace(schedule)
	command = strings.TrimSpace(command)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if len(name) > 120 {
		return "", fmt.Errorf("name is too long")
	}
	if err := ValidateSchedule(schedule); err != nil {
		return "", err
	}
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	if strings.ContainsRune(command, '\x00') || strings.Contains(command, "\n") || strings.Contains(command, "\r") {
		return "", fmt.Errorf("command must be a single line")
	}

	content, err := s.Read()
	if err != nil {
		return "", err
	}
	if id == "" {
		id = NewID()
		block := RenderManaged(ManagedBlock{ID: id, Name: name, Schedule: schedule, Command: command, Enabled: enabled}, s.RunnerPath)
		return id, s.Write(AppendBlock(content, block), "add-job")
	}
	if b, ok := FindManaged(content, id); ok {
		block := RenderManaged(ManagedBlock{ID: id, Name: name, Schedule: schedule, Command: command, Enabled: enabled}, s.RunnerPath)
		return id, s.Write(ReplaceRange(content, b.Start, b.End, block), "edit-job")
	}
	if r, ok := FindRaw(content, id); ok {
		newID := NewID()
		block := RenderManaged(ManagedBlock{ID: newID, Name: name, Schedule: schedule, Command: command, Enabled: enabled}, s.RunnerPath)
		return newID, s.Write(ReplaceLine(content, r.Index, block), "adopt-job")
	}
	return "", fmt.Errorf("job not found")
}

func (s *Store) ToggleJob(id string, enabled bool) error {
	content, err := s.Read()
	if err != nil {
		return err
	}
	if b, ok := FindManaged(content, id); ok {
		b.Enabled = enabled
		return s.Write(ReplaceRange(content, b.Start, b.End, RenderManaged(b, s.RunnerPath)), "toggle-job")
	}
	if r, ok := FindRaw(content, id); ok {
		b := ManagedBlock{ID: NewID(), Name: inferName(r.Command), Schedule: r.Schedule, Command: r.Command, Enabled: enabled}
		return s.Write(ReplaceLine(content, r.Index, RenderManaged(b, s.RunnerPath)), "adopt-toggle-job")
	}
	return fmt.Errorf("job not found")
}

func (s *Store) DeleteJob(id string) error {
	content, err := s.Read()
	if err != nil {
		return err
	}
	if b, ok := FindManaged(content, id); ok {
		return s.Write(ReplaceRange(content, b.Start, b.End, nil), "delete-job")
	}
	if r, ok := FindRaw(content, id); ok {
		return s.Write(ReplaceLine(content, r.Index, nil), "delete-imported-job")
	}
	return fmt.Errorf("job not found")
}

func (s *Store) ResolveJob(id string) (Job, error) {
	content, err := s.Read()
	if err != nil {
		return Job{}, err
	}
	if b, ok := FindManaged(content, id); ok {
		return Job{ID: b.ID, Name: b.Name, Schedule: b.Schedule, Command: b.Command, Enabled: b.Enabled, Managed: true, Human: Humanize(b.Schedule)}, nil
	}
	if r, ok := FindRaw(content, id); ok {
		return Job{ID: r.ID, Name: inferName(r.Command), Schedule: r.Schedule, Command: r.Command, Enabled: true, Managed: false, Human: Humanize(r.Schedule)}, nil
	}
	return Job{}, fmt.Errorf("job not found")
}

func (s *Store) UnmanageAll() error {
	content, err := s.Read()
	if err != nil {
		return err
	}
	managed, _ := Parse(content)
	if len(managed) == 0 {
		return nil
	}
	// Work from the end so original indices remain valid.
	for i := len(managed) - 1; i >= 0; i-- {
		b := managed[i]
		line := b.Schedule + " " + b.Command
		if !b.Enabled {
			line = "# " + line
		}
		content = ReplaceRange(content, b.Start, b.End, []string{line})
	}
	return s.Write(content, "unmanage-all")
}
