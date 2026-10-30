package flowingest

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ft "github.com/rayavriti/netmonitor-backend/internal/flowingest/flowtest"
	"github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type fakeStore struct {
	mu      sync.Mutex
	batches [][]models.Flow
	err     error
	devices []models.Device
	ctxErr  []error // ctx.Err() seen at each write
}

func (s *fakeStore) RecordFlows(ctx context.Context, flows []models.Flow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctxErr = append(s.ctxErr, ctx.Err())
	if s.err != nil {
		return s.err
	}
	s.batches = append(s.batches, append([]models.Flow(nil), flows...))
	return nil
}

func (s *fakeStore) GetDevices(context.Context) ([]models.Device, error) {
	return s.devices, nil
}

func (s *fakeStore) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.batches {
		n += len(b)
	}
	return n
}

func (s *fakeStore) all() []models.Flow {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []models.Flow
	for _, b := range s.batches {
		out = append(out, b...)
	}
	return out
}

func TestBatchWriter_FlushesOnTickerWhenIdle(t *testing.T) {
	store := &fakeStore{}
	w := newBatchWriter(store, 100, 1000, 20*time.Millisecond, time.Second)
	done := make(chan struct{})
	go func() { w.run(); close(done) }()

	w.enqueue(models.Flow{SrcIP: "10.0.0.1"})
	w.enqueue(models.Flow{SrcIP: "10.0.0.2"})
	require.Eventually(t, func() bool { return store.total() == 2 }, 2*time.Second, 5*time.Millisecond)

	// Several idle ticks later the ticker must still flush (a re-armed
	// timer that is only reset on receive would stall here).
	time.Sleep(100 * time.Millisecond)
	w.enqueue(models.Flow{SrcIP: "10.0.0.3"})
	require.Eventually(t, func() bool { return store.total() == 3 }, 2*time.Second, 5*time.Millisecond)

	close(w.queue)
	<-done
}

func TestBatchWriter_FlushesOnSize(t *testing.T) {
	store := &fakeStore{}
	w := newBatchWriter(store, 100, 3, time.Hour, time.Second)
	done := make(chan struct{})
	go func() { w.run(); close(done) }()
	for i := 0; i < 7; i++ {
		w.enqueue(models.Flow{})
	}
	require.Eventually(t, func() bool { return store.total() == 6 }, 2*time.Second, 5*time.Millisecond)

	// Closing the queue flushes the remainder with a live context.
	close(w.queue)
	<-done
	assert.Equal(t, 7, store.total())
	store.mu.Lock()
	defer store.mu.Unlock()
	assert.Len(t, store.batches, 3)
	for _, err := range store.ctxErr {
		assert.NoError(t, err)
	}
}

func TestBatchWriter_DropsWhenFull(t *testing.T) {
	w := newBatchWriter(&fakeStore{}, 2, 10, time.Hour, time.Second)
	before := testutil.ToFloat64(metrics.FlowRecordsDropped.WithLabelValues("queue_full"))
	assert.True(t, w.enqueue(models.Flow{}))
	assert.True(t, w.enqueue(models.Flow{}))
	assert.False(t, w.enqueue(models.Flow{}), "full queue drops instead of blocking")
	assert.False(t, w.enqueue(models.Flow{}))
	assert.Equal(t, before+2, testutil.ToFloat64(metrics.FlowRecordsDropped.WithLabelValues("queue_full")))
}

func TestBatchWriter_WriteErrorCounted(t *testing.T) {
	store := &fakeStore{err: errors.New("db down")}
	w := newBatchWriter(store, 10, 10, time.Hour, time.Second)
	dropped := testutil.ToFloat64(metrics.FlowRecordsDropped.WithLabelValues("write_error"))
	errs := testutil.ToFloat64(metrics.FlowBatchWriteErrors)
	w.enqueue(models.Flow{})
	w.enqueue(models.Flow{})
	close(w.queue)
	w.run()
	assert.Equal(t, dropped+2, testutil.ToFloat64(metrics.FlowRecordsDropped.WithLabelValues("write_error")))
	assert.Equal(t, errs+1, testutil.ToFloat64(metrics.FlowBatchWriteErrors))
}

func TestServer_EndToEndUDP(t *testing.T) {
	store := &fakeStore{devices: []models.Device{
		{ID: 9, IPAddress: "127.0.0.1"},
		{ID: 3, IPAddress: "127.0.0.1"}, // lowest ID wins
		{ID: 4, IPAddress: "router.example"},
	}}
	srv, err := New(Config{NetFlowAddr: "127.0.0.1:0", SFlowAddr: "127.0.0.1:0", FlushInterval: time.Hour}, store)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, srv.Start(ctx))
	nfAddr, sfAddr := srv.Addrs()

	v5Before := testutil.ToFloat64(metrics.FlowDatagramsTotal.WithLabelValues(protoNetFlowV5))
	decoded := func() float64 {
		return testutil.ToFloat64(metrics.FlowRecordsDecoded.WithLabelValues(protoNetFlowV5)) +
			testutil.ToFloat64(metrics.FlowRecordsDecoded.WithLabelValues(protoSFlow))
	}
	decodedBefore := decoded()
	send := func(addr net.Addr, b []byte) {
		c, err := net.Dial("udp", addr.String())
		require.NoError(t, err)
		defer c.Close()
		_, err = c.Write(b)
		require.NoError(t, err)
	}
	now := time.Now()
	send(nfAddr, ft.NetFlowV5(10_000, now, 1, 0, ft.V5Record{Src: "10.0.0.1", Dst: "10.0.0.2", Packets: 1, Bytes: 100, First: 9_000, Last: 10_000, Proto: 6}))
	send(sfAddr, ft.SFlowV5("127.0.0.1", 1, 1, ft.SFlowSample{SamplingRate: 10, FrameLength: 100,
		Header: ft.EthIPv4TCP("10.0.0.3", "10.0.0.4", 1, 2, 0x10, 0, 86)}))
	send(nfAddr, []byte("garbage"))
	require.Eventually(t, func() bool { return decoded() == decodedBefore+2 }, 2*time.Second, 5*time.Millisecond)
	assert.Zero(t, store.total(), "nothing written before the 1h ticker or Stop")

	// Stop drains the queue with a final flush even though the ticker
	// (1h) never fired.
	srv.Stop()
	flows := store.all()
	require.Len(t, flows, 2)
	for _, f := range flows {
		require.NotNil(t, f.DeviceID)
		assert.Equal(t, int64(3), *f.DeviceID)
	}
	assert.Equal(t, v5Before+1, testutil.ToFloat64(metrics.FlowDatagramsTotal.WithLabelValues(protoNetFlowV5)))
	srv.Stop() // idempotent
}

func TestServer_DisabledListenerAndBindError(t *testing.T) {
	srv, err := New(Config{NetFlowAddr: "127.0.0.1:0"}, &fakeStore{})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	nf, sf := srv.Addrs()
	assert.NotNil(t, nf)
	assert.Nil(t, sf)

	// A second server on the same port fails to start.
	srv2, err := New(Config{NetFlowAddr: nf.String()}, &fakeStore{})
	require.NoError(t, err)
	assert.Error(t, srv2.Start(context.Background()))
	srv2.Stop()

	srv.Stop()
}
