package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSchedule(t *testing.T) {
	valid := []string{"17 * * * *", "*/5 0-23 * JAN,MAR MON-FRI", "@reboot", "0 2 1 * *"}
	for _, v := range valid {
		if err := ValidateSchedule(v); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
	invalid := []string{"61 * * * *", "* 24 * * *", "* * 0 * *", "* * * *", "*/0 * * * *"}
	for _, v := range invalid {
		if err := ValidateSchedule(v); err == nil {
			t.Fatalf("expected invalid: %s", v)
		}
	}
}

func TestParseAndRenderManaged(t *testing.T) {
	b := ManagedBlock{ID: "abc123", Name: "Backup photos", Schedule: "17 * * * *", Command: "/bin/echo hello world", Enabled: true}
	text := JoinLines(RenderManaged(b, "/usr/local/AppCentral/CronManager/cron-manager"))
	m, raw := Parse(text)
	if len(m) != 1 || len(raw) != 0 {
		t.Fatalf("managed=%d raw=%d", len(m), len(raw))
	}
	if m[0].Name != b.Name || m[0].Command != b.Command || !m[0].Enabled {
		t.Fatalf("round-trip mismatch: %#v", m[0])
	}
}

func TestUnknownLinesRemainLiteral(t *testing.T) {
	original := "MAILTO=root\n# keep this comment exactly\n17 * * * * /opt/test --flag='a b'\n"
	_, raw := Parse(original)
	if len(raw) != 1 {
		t.Fatalf("expected raw job")
	}
	b := ManagedBlock{ID: "x1", Name: "Test", Schedule: raw[0].Schedule, Command: raw[0].Command, Enabled: true}
	changed := ReplaceLine(original, raw[0].Index, RenderManaged(b, "/runner"))
	if !strings.Contains(changed, "MAILTO=root\n# keep this comment exactly\n") {
		t.Fatalf("surrounding lines changed: %q", changed)
	}
}

func TestValidateExportBundle(t *testing.T) {
	b := ExportBundle{
		Format:        ExportFormat,
		FormatVersion: ExportFormatVersion,
		Jobs:          []PortableJob{{Name: "Hourly check", Schedule: "17 * * * *", Command: "/bin/true", Enabled: true}},
	}
	if err := ValidateExportBundle(b); err != nil {
		t.Fatal(err)
	}
	b.Jobs[0].Schedule = "99 * * * *"
	if err := ValidateExportBundle(b); err == nil {
		t.Fatal("expected invalid schedule")
	}
}

func TestInferNameSkipsAssignmentsAndShell(t *testing.T) {
	if got := inferName("TAG=CERTIFICATE /usr/builtin/bin/certificate update-cert"); got != "Imported: certificate" {
		t.Fatalf("got %q", got)
	}
	if got := inferName("/bin/sh /usr/builtin/sbin/ntpupdate.sh pool.ntp.org"); got != "Imported: ntpupdate.sh" {
		t.Fatalf("got %q", got)
	}
}

func TestHumanizeComplexMinuteFallsBack(t *testing.T) {
	const s = "0,10,15 0 * * *"
	if got := Humanize(s); got != s {
		t.Fatalf("got %q", got)
	}
}

func TestInferNameDockerExec(t *testing.T) {
	got := inferName("/usr/local/bin/docker exec example python3 /app/notifier/family_notifier.py >> /tmp/cron.log 2>&1")
	if got != "Imported: family_notifier.py" {
		t.Fatalf("got %q", got)
	}
}

func TestPortableImportPreservesRawCrontab(t *testing.T) {
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab.txt")
	original := "MAILTO=root\n# ADM entry stays literal\n0 0 * * * /usr/builtin/sbin/example --daily\n"
	if err := os.WriteFile(cronFile, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "crontab")
	sh := `#!/bin/sh
if [ "${1:-}" = "-l" ]; then cat "$FAKE_CRONTAB"; exit 0; fi
cat > "$FAKE_CRONTAB"
`
	if err := os.WriteFile(shim, []byte(sh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_CRONTAB", cronFile)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	store := NewStore(filepath.Join(dir, "backups"), "/runner")
	bundle := ExportBundle{
		Format: ExportFormat, FormatVersion: ExportFormatVersion, AppVersion: "1.0.0",
		Jobs: []PortableJob{{Name: "Portable", Schedule: "17 * * * *", Command: "/opt/portable.sh", Enabled: true}},
	}
	res, err := store.ImportManaged(bundle, "merge")
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 1 || res.Skipped != 0 {
		t.Fatalf("unexpected result: %#v", res)
	}
	got, err := os.ReadFile(cronFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "MAILTO=root\n# ADM entry stays literal\n0 0 * * * /usr/builtin/sbin/example --daily\n") {
		t.Fatalf("raw lines changed: %q", string(got))
	}
	if !strings.Contains(string(got), "# ACM-NAME ") || !strings.Contains(string(got), "/runner exec ") {
		t.Fatalf("managed job missing: %q", string(got))
	}
	backups, err := store.Backups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}

	exported, err := store.ExportManaged("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.Jobs) != 1 || exported.Jobs[0].Name != "Portable" {
		t.Fatalf("unexpected export: %#v", exported)
	}
}
