package timeutil

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"90m", 90 * time.Minute},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{" 30d ", 30 * 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if err != nil {
			t.Errorf("ParseDuration(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	for _, in := range []string{"", "notaduration", "24x", "d", "-d"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q): expected error", in)
		}
	}
}
