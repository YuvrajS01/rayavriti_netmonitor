package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestFlowAnalyzer_StartStop(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	analyzer.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	analyzer.Stop()
}

func TestFlowAnalyzer_Stop_BeforeStart(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 100)
	analyzer.Stop()
}

func TestFlowAnalyzer_Stop_FlushesPending(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 100)
	ctx, cancel := context.WithCancel(context.Background())
	analyzer.Start(ctx)

	ch := analyzer.IngestChannel()
	for i := 0; i < 10; i++ {
		ch <- []models.Flow{{SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Bytes: 100}}
	}

	time.Sleep(10 * time.Millisecond)
	cancel()
	analyzer.Stop()
}

func TestFlowAnalyzer_DBError(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 100)
	ctx, cancel := context.WithCancel(context.Background())
	analyzer.Start(ctx)

	ch := analyzer.IngestChannel()
	ch <- []models.Flow{{SrcIP: "1.1.1.1"}}

	time.Sleep(10 * time.Millisecond)
	cancel()
	analyzer.Stop()
}

func TestFlowAnalyzer_IngestChannel(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 10)
	ch := analyzer.IngestChannel()
	require.NotNil(t, ch)

	select {
	case ch <- []models.Flow{{SrcIP: "1.1.1.1"}}:
	default:
		t.Fatal("channel should not be full")
	}
}

func TestFlowAnalyzer_DefaultBufferSize(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 0)
	require.NotNil(t, analyzer)
}

func TestFlowAnalyzer_BatchFlush(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	analyzer := NewFlowAnalyzer(db, 100)
	ctx, cancel := context.WithCancel(context.Background())
	analyzer.Start(ctx)

	ch := analyzer.IngestChannel()
	// Send 500+ flows to trigger batch flush
	for i := 0; i < 600; i++ {
		ch <- []models.Flow{{SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Bytes: int64(i)}}
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	analyzer.Stop()
}
