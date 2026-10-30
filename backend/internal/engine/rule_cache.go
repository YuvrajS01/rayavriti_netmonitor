package engine

import (
	"context"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ruleCacheTTL bounds how stale cached rules can be. Edits on this instance
// invalidate immediately; the TTL covers edits made on other instances.
const ruleCacheTTL = 15 * time.Second

// ruleCache keeps the evaluable alert rules in memory. Loading them took
// three queries and ran for every poll result (B1).
type ruleCache struct {
	mu       sync.Mutex
	rules    []models.AlertRule
	loadedAt time.Time
	gen      uint64 // bumped on invalidate; a load started earlier is discarded
}

func (c *ruleCache) invalidate() {
	c.mu.Lock()
	c.gen++
	c.loadedAt = time.Time{}
	c.mu.Unlock()
}

// activeRules returns enabled rules that have conditions, reloading them
// when the cache is empty, invalidated or older than ruleCacheTTL. The
// returned slice is shared and must be treated as read-only.
func (e *AlertEngine) activeRules(ctx context.Context) ([]models.AlertRule, error) {
	c := &e.rules
	c.mu.Lock()
	if !c.loadedAt.IsZero() && time.Since(c.loadedAt) < ruleCacheTTL {
		rules := c.rules
		c.mu.Unlock()
		return rules, nil
	}
	gen := c.gen
	c.mu.Unlock()

	all, err := e.db.GetAlertRules(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]models.AlertRule, 0, len(all))
	for _, r := range all {
		if r.Enabled && len(r.Conditions) > 0 {
			active = append(active, r)
		}
	}

	c.mu.Lock()
	if c.gen == gen {
		c.rules = active
		c.loadedAt = time.Now()
	}
	c.mu.Unlock()
	return active, nil
}

// InvalidateRules drops cached rules so the next evaluation reloads them.
// Call it after any alert rule is created, updated, deleted or toggled.
func (e *AlertEngine) InvalidateRules() {
	e.rules.invalidate()
}
