package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AlertStateCache struct {
	rdb *Redis
	db  database.Database
}

func NewAlertStateCache(rdb *Redis, db database.Database) *AlertStateCache {
	return &AlertStateCache{rdb: rdb, db: db}
}

func (c *AlertStateCache) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	key := fmt.Sprintf("nm:alert:state:%d:%d", ruleID, deviceID)
	var state models.AlertRuleState
	if found, _ := c.rdb.Get(ctx, key, &state); found {
		return &state, nil
	}
	statePtr, err := c.db.GetAlertRuleState(ctx, ruleID, deviceID)
	if err != nil {
		return nil, err
	}
	if statePtr != nil {
		_ = c.rdb.Set(ctx, key, statePtr, 5*time.Minute)
	}
	return statePtr, nil
}

func (c *AlertStateCache) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	err := c.db.UpsertAlertRuleState(ctx, s)
	if err == nil {
		key := fmt.Sprintf("nm:alert:state:%d:%d", s.RuleID, s.DeviceID)
		_ = c.rdb.Set(ctx, key, s, 5*time.Minute)
	}
	return err
}

func (c *AlertStateCache) InvalidateAlertState(ctx context.Context, ruleID, deviceID int64) {
	key := fmt.Sprintf("nm:alert:state:%d:%d", ruleID, deviceID)
	_ = c.rdb.Del(ctx, key)
	slog.Debug("Alert state cache invalidated", "rule_id", ruleID, "device_id", deviceID)
}
