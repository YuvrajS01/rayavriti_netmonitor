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

func TestIsDue(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, kolkata)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	ptr := func(v time.Time) *time.Time { return &v }

	tests := []struct {
		name    string
		lastRun *time.Time
		cron    string
		now     time.Time
		want    bool
	}{
		{"never run is due", nil, "0 * * * *", at("2026-10-30 10:30"), true},
		{"daily not due an hour later", ptr(at("2026-10-30 00:00")), "0 0 * * *", at("2026-10-30 01:00"), false},
		{"daily due next midnight", ptr(at("2026-10-30 00:00")), "0 0 * * *", at("2026-10-31 00:00"), true},
		{"weekly not due next day", ptr(at("2026-10-26 00:00")), "0 0 * * 1", at("2026-10-27 00:00"), false},
		{"weekly due next monday", ptr(at("2026-10-26 00:00")), "0 0 * * 1", at("2026-11-02 00:01"), true},
		{"8am honours time of day", ptr(at("2026-10-30 08:00")), "0 8 * * *", at("2026-10-30 20:00"), false},
		{"descriptor", ptr(at("2026-10-30 00:00")), "@daily", at("2026-10-31 00:00"), true},
		{"every 5 minutes", ptr(at("2026-10-30 10:00")), "*/5 * * * *", at("2026-10-30 10:05"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isDue(tt.lastRun, tt.cron, "Asia/Kolkata", tt.now)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("isDue = %v, want %v", got, tt.want)
			}
		})
	}
	if _, err := isDue(nil, "not a cron", "", time.Now()); err == nil {
		t.Fatal("expected an error for an invalid expression")
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
