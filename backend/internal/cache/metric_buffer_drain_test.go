package cache

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fkViolationDB rejects metrics for device 13 the way Postgres does when the
// device has been deleted (SQLSTATE 23503).
type fkViolationDB struct {
	database.Database
	saved []int64
}

func (f *fkViolationDB) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	for _, m := range metrics {
		if m.DeviceID == 13 {
			return &pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"}
		}
	}
	for _, m := range metrics {
		f.saved = append(f.saved, m.DeviceID)
	}
	return nil
}

func (f *fkViolationDB) RecordMetric(ctx context.Context, m *models.Metric) error {
	if m.DeviceID == 13 {
		return &pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"}
	}
	f.saved = append(f.saved, m.DeviceID)
	return nil
}

func TestMetricBuffer_DrainEmptiesBacklogInOneCycle(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()
	_, _ = rdb.Client().Del(ctx, metricsBufferKey).Result()

	db := &succeedingDB{}
	b := NewMetricBuffer(rdb, db, 10, time.Second)
	for i := 0; i < 95; i++ {
		require.NoError(t, b.Push(ctx, &models.Metric{DeviceID: int64(i)}))
	}

	b.drain(ctx)

	llen, err := rdb.Client().LLen(ctx, metricsBufferKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), llen, "a backlog larger than one batch must drain in one cycle")
	assert.Equal(t, 10, db.batches)
}

func TestMetricBuffer_PermanentErrorsAreDeadLettered(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()
	_, _ = rdb.Client().Del(ctx, metricsBufferKey, deadLetterKey).Result()

	db := &fkViolationDB{}
	b := NewMetricBuffer(rdb, db, 10, time.Second)
	for _, id := range []int64{11, 12, 13, 14} {
		require.NoError(t, b.Push(ctx, &models.Metric{DeviceID: id}))
	}

	b.drain(ctx)
	b.drain(ctx) // a poison row must not come back

	assert.ElementsMatch(t, []int64{11, 12, 14}, db.saved)
	queued, _ := rdb.Client().LLen(ctx, metricsBufferKey).Result()
	dead, _ := rdb.Client().LLen(ctx, deadLetterKey).Result()
	assert.Equal(t, int64(0), queued)
	assert.Equal(t, int64(1), dead)
}
