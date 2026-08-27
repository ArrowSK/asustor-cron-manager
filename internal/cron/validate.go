package cron

import (
	"fmt"
	"strconv"
	"strings"
)

var macros = map[string]bool{
	"@reboot": true, "@yearly": true, "@annually": true, "@monthly": true,
	"@weekly": true, "@daily": true, "@midnight": true, "@hourly": true,
}

var monthNames = map[string]int{"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12}
var dayNames = map[string]int{"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6}

type fieldSpec struct {
	min, max int
	names    map[string]int
}

func ValidateSchedule(schedule string) error {
	schedule = strings.TrimSpace(schedule)
	if macros[schedule] {
		return nil
	}
	parts := strings.Fields(schedule)
	if len(parts) != 5 {
		return fmt.Errorf("schedule must contain 5 cron fields or a supported @macro")
	}
	specs := []fieldSpec{{0, 59, nil}, {0, 23, nil}, {1, 31, nil}, {1, 12, monthNames}, {0, 7, dayNames}}
	for i, p := range parts {
		if err := validateField(p, specs[i]); err != nil {
			return fmt.Errorf("field %d (%q): %w", i+1, p, err)
		}
	}
	return nil
}

func validateField(field string, spec fieldSpec) error {
	if field == "" {
		return fmt.Errorf("empty field")
	}
	for _, item := range strings.Split(field, ",") {
		if item == "" {
			return fmt.Errorf("empty list item")
		}
		base := item
		if strings.Contains(item, "/") {
			parts := strings.Split(item, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid step syntax")
			}
			base = parts[0]
			step, err := strconv.Atoi(parts[1])
			if err != nil || step <= 0 {
				return fmt.Errorf("step must be a positive integer")
			}
		}
		if base == "*" {
			continue
		}
		if strings.Contains(base, "-") {
			r := strings.Split(base, "-")
			if len(r) != 2 {
				return fmt.Errorf("invalid range")
			}
			a, err := parseValue(r[0], spec)
			if err != nil {
				return err
			}
			b, err := parseValue(r[1], spec)
			if err != nil {
				return err
			}
			if a > b {
				return fmt.Errorf("range start must be <= end")
			}
			continue
		}
		if _, err := parseValue(base, spec); err != nil {
			return err
		}
	}
	return nil
}

func parseValue(s string, spec fieldSpec) (int, error) {
	up := strings.ToUpper(s)
	if spec.names != nil {
		if v, ok := spec.names[up]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid value", s)
	}
	if v < spec.min || v > spec.max {
		return 0, fmt.Errorf("%d is outside %d-%d", v, spec.min, spec.max)
	}
	return v, nil
}

func Humanize(schedule string) string {
	s := strings.TrimSpace(schedule)
	switch s {
	case "@reboot":
		return "At system startup"
	case "@hourly":
		return "Every hour"
	case "@daily", "@midnight":
		return "Every day"
	case "@weekly":
		return "Every week"
	case "@monthly":
		return "Every month"
	case "@yearly", "@annually":
		return "Every year"
	}
	p := strings.Fields(s)
	if len(p) != 5 {
		return s
	}
	if p[0] == "*" && p[1] == "*" && p[2] == "*" && p[3] == "*" && p[4] == "*" {
		return "Every minute"
	}
	if strings.HasPrefix(p[0], "*/") && p[1] == "*" && p[2] == "*" && p[3] == "*" && p[4] == "*" {
		return "Every " + strings.TrimPrefix(p[0], "*/") + " minutes"
	}
	if isPlainNumber(p[0]) && p[1] == "*" && p[2] == "*" && p[3] == "*" && p[4] == "*" {
		return fmt.Sprintf("Every hour at :%02s", p[0])
	}
	if isPlainNumber(p[0]) && isPlainNumber(p[1]) && p[2] == "*" && p[3] == "*" && p[4] == "*" {
		return fmt.Sprintf("Every day at %02s:%02s", p[1], p[0])
	}
	if isPlainNumber(p[0]) && isPlainNumber(p[1]) && p[2] == "*" && p[3] == "*" && p[4] != "*" {
		return fmt.Sprintf("Weekly (%s) at %02s:%02s", p[4], p[1], p[0])
	}
	if isPlainNumber(p[0]) && isPlainNumber(p[1]) && p[2] != "*" && p[3] == "*" && p[4] == "*" {
		return fmt.Sprintf("Monthly on day %s at %02s:%02s", p[2], p[1], p[0])
	}
	return s
}

func isPlainNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
