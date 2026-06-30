package monitoring

import (
	"context"
	"sync"

	"github.com/rayavriti/netmonitor-backend/internal/logging"
)

type AsyncLogSink struct {
	store *Store
	ch    chan logging.PersistedEvent
	wg    sync.WaitGroup
}

func NewAsyncLogSink(store *Store, queueSize int) *AsyncLogSink {
	if queueSize <= 0 {
		queueSize = 10000
	}
	return &AsyncLogSink{store: store, ch: make(chan logging.PersistedEvent, queueSize)}
}

func (s *AsyncLogSink) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-s.ch:
				_ = s.store.RecordLogEvent(context.Background(), evt)
			}
		}
	}()
}

func (s *AsyncLogSink) Persist(ctx context.Context, evt logging.PersistedEvent) {
	select {
	case s.ch <- evt:
	default:
		if evt.Level == "debug" || evt.Level == "trace" {
			return
		}
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- evt:
		default:
		}
	}
}
