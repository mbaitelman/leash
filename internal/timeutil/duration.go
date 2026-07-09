// Package timeutil provides shared time helpers used across leash packages.
package timeutil

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dayPattern = regexp.MustCompile(`^(\d+)d$`)

// ParseDuration supports Go durations plus a "d" (day) suffix.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if m := dayPattern.FindStringSubmatch(s); m != nil {
		days, _ := strconv.Atoi(m[1])
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
