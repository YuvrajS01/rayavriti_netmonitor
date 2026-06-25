package engine

import (
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestResolveTarget_PreferredChannel(t *testing.T) {
	tests := []struct {
		name     string
		contact  models.Contact
		expected string
	}{
		{
			name: "telegram preferred",
			contact: models.Contact{
				PreferredChannel: "telegram",
				TelegramChatID:   "123456",
				Email:            "test@example.com",
			},
			expected: "123456",
		},
		{
			name: "email preferred",
			contact: models.Contact{
				PreferredChannel: "email",
				Email:            "test@example.com",
			},
			expected: "test@example.com",
		},
		{
			name: "whatsapp preferred",
			contact: models.Contact{
				PreferredChannel: "whatsapp",
				WhatsAppNumber:   "+1234567890",
			},
			expected: "+1234567890",
		},
		{
			name: "sms preferred",
			contact: models.Contact{
				PreferredChannel: "sms",
				Phone:            "+1234567890",
			},
			expected: "+1234567890",
		},
		{
			name: "fallback to telegram",
			contact: models.Contact{
				PreferredChannel: "telegram",
				TelegramChatID:   "",
				Email:            "test@example.com",
			},
			expected: "test@example.com",
		},
		{
			name:     "no channels",
			contact:  models.Contact{},
			expected: "",
		},
		{
			name: "fallback order: telegram > email > phone > whatsapp",
			contact: models.Contact{
				Email:          "test@example.com",
				Phone:          "+123456",
				WhatsAppNumber: "+789012",
			},
			expected: "test@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTarget(tt.contact)
			if got != tt.expected {
				t.Errorf("resolveTarget() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		notifyOn string
		severity string
		want     bool
	}{
		{"empty notify_on matches all", "", "critical", true},
		{"matches critical", "critical,warning", "critical", true},
		{"matches warning", "critical,warning", "warning", true},
		{"does not match info", "critical,warning", "info", false},
		{"single value match", "critical", "critical", true},
		{"single value no match", "critical", "info", false},
		{"with spaces", "critical, warning", "warning", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldNotify(tt.notifyOn, tt.severity)
			if got != tt.want {
				t.Errorf("shouldNotify(%q, %q) = %v, want %v", tt.notifyOn, tt.severity, got, tt.want)
			}
		})
	}
}

func TestInQuietHours(t *testing.T) {
	now := time.Now()
	currentHour := now.Hour()
	currentMin := now.Minute()
	quietStart := time.Date(now.Year(), now.Month(), now.Day(), currentHour, 0, 0, 0, time.Local)
	quietEnd := time.Date(now.Year(), now.Month(), now.Day(), currentHour+2, 0, 0, 0, time.Local)

	startStr := quietStart.Format("15:04")
	endStr := quietEnd.Format("15:04")

	s := startStr
	e := endStr
	if !inQuietHours(&s, &e) {
		t.Error("expected to be in quiet hours")
	}

	farFuture := time.Date(now.Year(), now.Month(), now.Day(), (currentHour+5)%24, 0, 0, 0, time.Local)
	farEnd := time.Date(now.Year(), now.Month(), now.Day(), (currentHour+6)%24, 0, 0, 0, time.Local)
	s2 := farFuture.Format("15:04")
	e2 := farEnd.Format("15:04")
	if inQuietHours(&s2, &e2) {
		t.Error("expected not to be in quiet hours")
	}

	if inQuietHours(nil, &e) {
		t.Error("nil start should not be in quiet hours")
	}
	if inQuietHours(&s, nil) {
		t.Error("nil end should not be in quiet hours")
	}

	bad := "xx:xx"
	if inQuietHours(&bad, &e) {
		t.Error("bad format should not be in quiet hours")
	}

	_ = currentMin
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"00:00", 0},
		{"12:30", 750},
		{"23:59", 1439},
		{"xx:xx", -1},
		{"9:00", -1},
		{"12:00", 720},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseHHMM(tt.input)
			if got != tt.want {
				t.Errorf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLocationIDOrZero(t *testing.T) {
	if got := locationIDOrZero(nil); got != 0 {
		t.Errorf("nil should return 0, got %d", got)
	}
	v := int64(42)
	if got := locationIDOrZero(&v); got != 42 {
		t.Errorf("42 should return 42, got %d", got)
	}
}

func TestSplitComma(t *testing.T) {
	got := splitComma("critical, warning , info")
	expected := []string{"critical", "warning", "info"}
	if len(got) != len(expected) {
		t.Fatalf("len = %d, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], expected[i])
		}
	}

	got2 := splitComma("")
	if len(got2) != 0 {
		t.Errorf("empty should return empty slice, got %v", got2)
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"  hello  ", "hello"},
		{"no spaces", "no spaces"},
		{"   ", ""},
		{"", ""},
		{" x ", "x"},
	}
	for _, tt := range tests {
		if got := trimSpace(tt.input); got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
