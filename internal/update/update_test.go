package update

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.1", "1.0.0", 1},
		{"v1.2.0", "1.10.0", -1},
		{"2.0", "2.0.0", 0},
		{"1.0.0", "1.0.0", 0},
	}
	for _, tc := range cases {
		got := compareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	const sum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, err := parseChecksum(sum + "  cron-manager_linux_arm64\n")
	if err != nil || got != sum {
		t.Fatalf("got %q err=%v", got, err)
	}
}
