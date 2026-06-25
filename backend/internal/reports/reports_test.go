package reports

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{30, "30s"},
		{90, "1m 30s"},
		{3600, "1h 0m"},
		{5400, "1h 30m"},
		{0, "0s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"0 * * * *", "0 *", true},
		{"*/5 * * * *", "*/5 *", true},
		{"0 0 * * 1", "0 0 * * 1", true},
		{"daily", "*/5 *", false},
		{"", "", true},
	}
	for _, tt := range tests {
		got := containsSubstring(tt.s, tt.sub)
		if got != tt.want {
			t.Errorf("containsSubstring(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("*/5 * * * *", "*/5 *") {
		t.Error("expected true for */5 * * * *")
	}
	if containsAny("0 0 * * 1", "*/5 *") {
		t.Error("expected false for 0 0 * * 1")
	}
}

func TestNewScheduledRunner(t *testing.T) {
	r := NewScheduledRunner(nil, nil, 0)
	if r.interval != time.Minute {
		t.Errorf("expected default interval 1m, got %v", r.interval)
	}
	r2 := NewScheduledRunner(nil, nil, 5*time.Minute)
	if r2.interval != 5*time.Minute {
		t.Errorf("expected 5m interval, got %v", r2.interval)
	}
}

func TestComputePeriod(t *testing.T) {
	r := &ScheduledRunner{}
	from, to := r.computePeriod(nil, nil, "7d")
	if from.After(to) {
		t.Error("from should be before to")
	}
	if to.Sub(from).Hours() > 8*24 {
		t.Error("7d period should be at most 8 days")
	}
}

func TestIsDueNeverRun(t *testing.T) {
	r := &ScheduledRunner{}
	if !r.isDue(nil, "0 * * * *") {
		t.Error("should be due when never run")
	}
}

func TestNewISPCollector(t *testing.T) {
	c := NewISPCollector(nil, 0)
	if c.interval != 10*time.Second {
		t.Errorf("expected 10s default, got %v", c.interval)
	}
	c2 := NewISPCollector(nil, 30)
	if c2.interval != 30*time.Second {
		t.Errorf("expected 30s, got %v", c2.interval)
	}
}
