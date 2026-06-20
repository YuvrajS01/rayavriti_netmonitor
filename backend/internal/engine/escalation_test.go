package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"hi", 2, "hi"},
		{"", 5, ""},
		{"exactly", 7, "exactly"},
	}

	for _, tt := range tests {
		if got := truncate(tt.input, tt.max); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestNewEscalationEngine(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, nil)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.running == nil {
		t.Fatal("expected running map to be initialized")
	}
	if engine.config == nil {
		t.Fatal("expected default config")
	}
	if engine.config.MaxSteps != 0 {
		t.Error("expected zero MaxSteps in default config")
	}
}

func TestNewEscalationEngineWithConfig(t *testing.T) {
	cfg := &EscalationConfig{
		Enabled:         true,
		BotToken:        "test-token",
		DefaultChatID:   "12345",
		MaxSteps:        5,
		DefaultDelayMin: 15,
	}
	engine := NewEscalationEngine(nil, nil, nil, cfg)
	if !engine.config.Enabled {
		t.Error("expected enabled")
	}
	if engine.config.BotToken != "test-token" {
		t.Errorf("expected bot token 'test-token', got %q", engine.config.BotToken)
	}
}

func TestCancelEscalation(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, nil)
	engine.running[100] = &escalationRun{alertID: 100, step: 0}

	engine.CancelEscalation(100)

	if _, ok := engine.running[100]; ok {
		t.Error("expected escalation to be removed")
	}
}

func TestCancelEscalationNotFound(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, nil)
	engine.running[100] = &escalationRun{alertID: 100, step: 0}

	engine.CancelEscalation(999)

	if _, ok := engine.running[100]; !ok {
		t.Error("expected escalation 100 to still exist")
	}
}

func TestGetActiveStep(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, nil)
	engine.running[100] = &escalationRun{alertID: 100, step: 2}

	if got := engine.GetActiveStep(100); got != 2 {
		t.Errorf("GetActiveStep(100) = %d, want 2", got)
	}

	if got := engine.GetActiveStep(999); got != -1 {
		t.Errorf("GetActiveStep(999) = %d, want -1", got)
	}
}

func TestRunCount(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, nil)
	if got := engine.RunCount(); got != 0 {
		t.Errorf("RunCount() = %d, want 0", got)
	}

	engine.running[1] = &escalationRun{alertID: 1, step: 0}
	engine.running[2] = &escalationRun{alertID: 2, step: 1}
	if got := engine.RunCount(); got != 2 {
		t.Errorf("RunCount() = %d, want 2", got)
	}
}

func TestStartEscalation_Disabled(t *testing.T) {
	engine := NewEscalationEngine(nil, nil, nil, &EscalationConfig{Enabled: false})
	err := engine.StartEscalation(context.Background(), &models.Alert{ID: 1}, 1)
	if err != nil {
		t.Errorf("expected nil error when disabled, got %v", err)
	}
}

func TestEscalationRun_Cancelled(t *testing.T) {
	run := &escalationRun{alertID: 1, step: 0, cancelled: false}
	run.cancelled = true
	if !run.cancelled {
		t.Error("expected cancelled to be true")
	}
}

func TestEscalationConfig_Defaults(t *testing.T) {
	cfg := &EscalationConfig{}
	if cfg.Enabled {
		t.Error("expected disabled by default")
	}
	if cfg.MaxSteps != 0 {
		t.Error("expected zero MaxSteps")
	}
	if cfg.DefaultDelayMin != 0 {
		t.Error("expected zero DefaultDelayMin")
	}
}

func TestTruncate_EmptyString(t *testing.T) {
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate empty string, got %q", got)
	}
}

func TestTruncate_ExactlyMax(t *testing.T) {
	s := "12345"
	if got := truncate(s, 5); got != "12345" {
		t.Errorf("expected exact match, got %q", got)
	}
}

func TestTimeNow_Used(t *testing.T) {
	_ = time.Now()
}
