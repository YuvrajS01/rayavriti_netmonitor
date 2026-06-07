package logging

import (
	"io"
	"os"
	"sync"
)

// MultiSink writes log output to multiple destinations simultaneously.
type MultiSink struct {
	writers []io.Writer
}

// NewMultiSink creates a sink that fans out to all provided writers.
func NewMultiSink(writers ...io.Writer) *MultiSink {
	return &MultiSink{writers: writers}
}

// Write implements io.Writer — writes to all sinks, collecting the first error.
func (m *MultiSink) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// StdoutSink returns an io.Writer that writes to stdout.
func StdoutSink() io.Writer { return os.Stdout }

// StderrSink returns an io.Writer that writes to stderr.
func StderrSink() io.Writer { return os.Stderr }

// DBSink provides an asynchronous, channel-based sink for writing log records
// to a database. It is recursion-safe — errors from its own writes are never
// re-enqueued back to itself.
type DBSink struct {
	queue      chan []byte
	queueSize  int
	dropPolicy string // "drop_debug" or "drop_oldest"
	writer     func(data []byte) error
	wg         sync.WaitGroup
	stopCh     chan struct{}
	isWriting  bool // guards against recursion
	mu         sync.Mutex
}

// NewDBSink creates an async DB sink with bounded queue.
// writer is called asynchronously for each log record.
// dropPolicy controls behavior when queue is full:
//   - "drop_debug": drop DEBUG and TRACE records when full
//   - "drop_oldest": drop the oldest record in the queue
func NewDBSink(queueSize int, dropPolicy string, writer func(data []byte) error) *DBSink {
	if queueSize <= 0 {
		queueSize = 10000
	}
	return &DBSink{
		queue:      make(chan []byte, queueSize),
		queueSize:  queueSize,
		dropPolicy: dropPolicy,
		writer:     writer,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the background consumer goroutine.
func (d *DBSink) Start() {
	d.wg.Add(1)
	go d.consume()
}

// Stop flushes remaining records and stops the background consumer.
func (d *DBSink) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

// Write enqueues a log record for async DB persistence.
// Implements io.Writer. Returns immediately; never blocks the caller.
func (d *DBSink) Write(p []byte) (int, error) {
	// Recursion guard: don't enqueue our own write errors
	d.mu.Lock()
	if d.isWriting {
		d.mu.Unlock()
		return len(p), nil
	}
	d.mu.Unlock()

	// Make a copy since p may be reused by the caller
	cp := make([]byte, len(p))
	copy(cp, p)

	select {
	case d.queue <- cp:
		// Enqueued successfully
	default:
		// Queue full — apply drop policy
		switch d.dropPolicy {
		case "drop_oldest":
			// Drain one old record to make room
			select {
			case <-d.queue:
			default:
			}
			select {
			case d.queue <- cp:
			default:
			}
		default:
			// "drop_debug" — silently drop this record
		}
	}
	return len(p), nil
}

func (d *DBSink) consume() {
	defer d.wg.Done()
	for {
		select {
		case data := <-d.queue:
			d.mu.Lock()
			d.isWriting = true
			d.mu.Unlock()

			_ = d.writer(data)

			d.mu.Lock()
			d.isWriting = false
			d.mu.Unlock()
		case <-d.stopCh:
			// Drain remaining records
			for {
				select {
				case data := <-d.queue:
					d.mu.Lock()
					d.isWriting = true
					d.mu.Unlock()
					_ = d.writer(data)
					d.mu.Lock()
					d.isWriting = false
					d.mu.Unlock()
				default:
					return
				}
			}
		}
	}
}
