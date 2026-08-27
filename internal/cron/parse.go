package cron

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	beginPrefix    = "# ACM-BEGIN "
	endPrefix      = "# ACM-END "
	namePrefix     = "# ACM-NAME "
	schedulePrefix = "# ACM-SCHEDULE "
	commandPrefix  = "# ACM-COMMAND "
	enabledPrefix  = "# ACM-ENABLED "
)

func encodeMeta(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
func decodeMeta(s string) string {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return ""
	}
	return string(b)
}

func RenderManaged(b ManagedBlock, runnerPath string) []string {
	lines := []string{
		beginPrefix + b.ID,
		namePrefix + encodeMeta(b.Name),
		schedulePrefix + encodeMeta(b.Schedule),
		commandPrefix + encodeMeta(b.Command),
	}
	if b.Enabled {
		lines = append(lines, enabledPrefix+"true", fmt.Sprintf("%s %s exec %s", b.Schedule, runnerPath, b.ID))
	} else {
		lines = append(lines, enabledPrefix+"false", "# ACM-DISABLED")
	}
	lines = append(lines, endPrefix+b.ID)
	return lines
}

func Parse(content string) (managed []ManagedBlock, raw []RawJob) {
	lines := splitLines(content)
	inBlock := false
	var b ManagedBlock
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, beginPrefix) {
			inBlock = true
			b = ManagedBlock{ID: strings.TrimSpace(strings.TrimPrefix(line, beginPrefix)), Start: i, End: i}
			continue
		}
		if inBlock {
			b.End = i
			switch {
			case strings.HasPrefix(line, namePrefix):
				b.Name = decodeMeta(strings.TrimPrefix(line, namePrefix))
			case strings.HasPrefix(line, schedulePrefix):
				b.Schedule = decodeMeta(strings.TrimPrefix(line, schedulePrefix))
			case strings.HasPrefix(line, commandPrefix):
				b.Command = decodeMeta(strings.TrimPrefix(line, commandPrefix))
			case strings.HasPrefix(line, enabledPrefix):
				b.Enabled = strings.TrimSpace(strings.TrimPrefix(line, enabledPrefix)) == "true"
			case strings.HasPrefix(line, endPrefix):
				if b.ID != "" && b.Schedule != "" && b.Command != "" {
					managed = append(managed, b)
				}
				inBlock = false
			}
			continue
		}

		if schedule, command, ok := ParseCronLine(line); ok {
			h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", i, line)))
			raw = append(raw, RawJob{
				ID:       fmt.Sprintf("raw:%x", h[:8]),
				Schedule: schedule,
				Command:  command,
				Line:     line,
				Index:    i,
			})
		}
	}
	return managed, raw
}

func ParseCronLine(line string) (schedule, command string, ok bool) {
	trim := strings.TrimSpace(line)
	if trim == "" || strings.HasPrefix(trim, "#") || isEnvLine(trim) {
		return "", "", false
	}
	parts := strings.Fields(trim)
	if len(parts) < 2 {
		return "", "", false
	}
	if strings.HasPrefix(parts[0], "@") {
		if !macros[parts[0]] || len(parts) < 2 {
			return "", "", false
		}
		idx := strings.Index(trim, parts[1])
		if idx < 0 {
			return "", "", false
		}
		return parts[0], strings.TrimSpace(trim[idx:]), true
	}
	if len(parts) < 6 {
		return "", "", false
	}
	schedule = strings.Join(parts[:5], " ")
	if ValidateSchedule(schedule) != nil {
		return "", "", false
	}
	// Find command without normalizing its internal whitespace.
	pos := 0
	for n := 0; n < 5; n++ {
		for pos < len(trim) && (trim[pos] == ' ' || trim[pos] == '\t') {
			pos++
		}
		for pos < len(trim) && trim[pos] != ' ' && trim[pos] != '\t' {
			pos++
		}
	}
	for pos < len(trim) && (trim[pos] == ' ' || trim[pos] == '\t') {
		pos++
	}
	if pos >= len(trim) {
		return "", "", false
	}
	return schedule, trim[pos:], true
}

func isEnvLine(s string) bool {
	if strings.ContainsAny(s, " \t") {
		first := strings.Fields(s)[0]
		if strings.Contains(first, "=") {
			return true
		}
	}
	if eq := strings.IndexByte(s, '='); eq > 0 {
		name := s[:eq]
		for i, r := range name {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
				return false
			}
		}
		return true
	}
	return false
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

func JoinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func FindManaged(content, id string) (ManagedBlock, bool) {
	m, _ := Parse(content)
	for _, b := range m {
		if b.ID == id {
			return b, true
		}
	}
	return ManagedBlock{}, false
}

func FindRaw(content, id string) (RawJob, bool) {
	_, r := Parse(content)
	for _, j := range r {
		if j.ID == id {
			return j, true
		}
	}
	return RawJob{}, false
}

func ReplaceRange(content string, start, end int, replacement []string) string {
	lines := splitLines(content)
	if start < 0 || end < start || end >= len(lines) {
		return content
	}
	out := make([]string, 0, len(lines)-(end-start+1)+len(replacement))
	out = append(out, lines[:start]...)
	out = append(out, replacement...)
	out = append(out, lines[end+1:]...)
	return JoinLines(out)
}

func ReplaceLine(content string, index int, replacement []string) string {
	return ReplaceRange(content, index, index, replacement)
}

func AppendBlock(content string, block []string) string {
	lines := splitLines(content)
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, block...)
	return JoinLines(lines)
}
