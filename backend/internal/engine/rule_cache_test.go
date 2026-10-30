package engine

import (
	"context"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestActiveRules_CachesAndInvalidates(t *testing.T) {
	loads := 0
	rules := []models.AlertRule{
		{ID: 1, Enabled: true, Conditions: []models.AlertRuleCondition{{}}},
		{ID: 2, Enabled: false, Conditions: []models.AlertRuleCondition{{}}},
		{ID: 3, Enabled: true},
	}
	db := &mockDB{getAlertRulesFn: func(context.Context) ([]models.AlertRule, error) {
		loads++
		return rules, nil
	}}
	e := NewAlertEngine(db, nil, nil)
	ctx := context.Background()

	got, err := e.activeRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("activeRules = %+v, want only the enabled rule with conditions", got)
	}
	for i := 0; i < 100; i++ {
		_, _ = e.activeRules(ctx)
	}
	if loads != 1 {
		t.Fatalf("rules loaded %d times within the TTL, want 1", loads)
	}

	e.InvalidateRules()
	_, _ = e.activeRules(ctx)
	if loads != 2 {
		t.Fatalf("rules loaded %d times after invalidation, want 2", loads)
	}
}
