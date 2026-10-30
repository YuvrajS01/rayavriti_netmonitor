// Package flowingest receives NetFlow v5/v9, IPFIX and sFlow v5 over UDP,
// normalises the records (end-time timestamps, sampling-rate scaling,
// exporter and interface metadata) and writes them to the flows hypertable
// in batches. Decoding uses goflow2's decoders and protobuf producer.
package flowingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// Store is what the ingest server needs from the database.
type Store interface {
	FlowWriter
	GetDevices(ctx context.Context) ([]models.Device, error)
}

// Config configures the ingest server. Zero values take the defaults below.
type Config struct {
	NetFlowAddr string // NetFlow v5/v9 + IPFIX; "" disables
	SFlowAddr   string // sFlow v5; "" disables

	QueueSize     int           // flows buffered for the writer (default 65536)
	BatchSize     int           // flows per COPY (default 2000)
	FlushInterval time.Duration // ticker flush (default 2s)
	WriteTimeout  time.Duration // per-batch write timeout (default 10s)
	DeviceRefresh time.Duration // exporter IP -> device reload (default 1m)
	// MaxAge bounds how old an exporter-supplied flow time may be (normally
	// the flows retention); older times fall back to the receive time.
	MaxAge       time.Duration // default 7 days
	MaxExporters int           // v9/IPFIX exporters with template state (default 4096)
	ReadBuffer   int           // UDP socket receive buffer bytes (default 4 MiB)
}

func (c *Config) setDefaults() {
	if c.QueueSize <= 0 {
		c.QueueSize = 65536
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 2000
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 2 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.DeviceRefresh <= 0 {
		c.DeviceRefresh = time.Minute
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 7 * 24 * time.Hour
	}
	if c.MaxExporters <= 0 {
		c.MaxExporters = 4096
	}
	if c.ReadBuffer <= 0 {
		c.ReadBuffer = 4 << 20
	}
}

// Server owns the UDP listeners, the decoder and the batch writer.
type Server struct {
	cfg     Config
	store   Store
	dec     *decoder
	writer  *batchWriter
	devices atomic.Pointer[map[netip.Addr]int64]

	cancel context.CancelFunc
	done   chan struct{}

	netflowConn, sflowConn *net.UDPConn
}

// New builds a server; call Start to bind and run it.
func New(cfg Config, store Store) (*Server, error) {
	cfg.setDefaults()
	s := &Server{cfg: cfg, store: store}
	dec, err := newDecoder(cfg.MaxExporters, cfg.MaxAge, s.lookupDevice)
	if err != nil {
		return nil, err
	}
	s.dec = dec
	s.writer = newBatchWriter(store, cfg.QueueSize, cfg.BatchSize, cfg.FlushInterval, cfg.WriteTimeout)
	return s, nil
}

// Start binds the configured listeners (an error here means nothing was
// started) and runs until ctx is cancelled or Stop is called.
func (s *Server) Start(ctx context.Context) error {
	var err error
	if s.cfg.NetFlowAddr != "" {
		if s.netflowConn, err = s.listen(s.cfg.NetFlowAddr); err != nil {
			return fmt.Errorf("netflow listen %s: %w", s.cfg.NetFlowAddr, err)
		}
	}
	if s.cfg.SFlowAddr != "" {
		if s.sflowConn, err = s.listen(s.cfg.SFlowAddr); err != nil {
			if s.netflowConn != nil {
				_ = s.netflowConn.Close()
			}
			return fmt.Errorf("sflow listen %s: %w", s.cfg.SFlowAddr, err)
		}
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	s.refreshDevices(ctx)

	var readers sync.WaitGroup
	if s.netflowConn != nil {
		readers.Add(1)
		go s.readLoop(ctx, &readers, s.netflowConn, false)
		slog.Info("Flow ingest: NetFlow/IPFIX listening", "addr", s.netflowConn.LocalAddr().String())
	}
	if s.sflowConn != nil {
		readers.Add(1)
		go s.readLoop(ctx, &readers, s.sflowConn, true)
		slog.Info("Flow ingest: sFlow listening", "addr", s.sflowConn.LocalAddr().String())
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.writer.run()
	}()
	go s.deviceRefreshLoop(ctx)

	// Shutdown: close sockets to unblock the readers, wait for them (the
	// only senders) before closing the queue, then let the writer drain.
	go func() {
		defer close(s.done)
		<-ctx.Done()
		if s.netflowConn != nil {
			_ = s.netflowConn.Close()
		}
		if s.sflowConn != nil {
			_ = s.sflowConn.Close()
		}
		readers.Wait()
		close(s.writer.queue)
		<-writerDone
	}()
	return nil
}

// Stop shuts the server down and waits for the final flush.
func (s *Server) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
}

// QueueLen reports flows waiting for the writer.
func (s *Server) QueueLen() int { return len(s.writer.queue) }

// Addrs returns the bound listener addresses (nil when disabled).
func (s *Server) Addrs() (netflow, sflow net.Addr) {
	if s.netflowConn != nil {
		netflow = s.netflowConn.LocalAddr()
	}
	if s.sflowConn != nil {
		sflow = s.sflowConn.LocalAddr()
	}
	return netflow, sflow
}

func (s *Server) listen(addr string) (*net.UDPConn, error) {
	ua, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", ua)
	if err != nil {
		return nil, err
	}
	if err := conn.SetReadBuffer(s.cfg.ReadBuffer); err != nil {
		slog.Warn("Flow ingest: could not set UDP receive buffer", "addr", addr, "error", err)
	}
	return conn, nil
}

// readLoop reads datagrams and hands decoded flows to the writer without
// ever blocking on it.
func (s *Server) readLoop(ctx context.Context, wg *sync.WaitGroup, conn *net.UDPConn, isSFlow bool) {
	defer wg.Done()
	buf := make([]byte, 65535)
	for {
		n, from, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			slog.Warn("Flow ingest: UDP read error", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		s.handleDatagram(buf[:n], from.Addr().Unmap(), time.Now(), isSFlow)
	}
}

// handleDatagram decodes one datagram and enqueues its flows. The decoded
// flows hold no references into data, so the read buffer can be reused.
func (s *Server) handleDatagram(data []byte, src netip.Addr, recv time.Time, isSFlow bool) {
	defer func() {
		if r := recover(); r != nil {
			metrics.FlowDecodeErrors.WithLabelValues(protoUnknown, "panic").Inc()
			slog.Error("Flow ingest: panic decoding datagram", "panic", r, "exporter", src.String(), "stack", string(debug.Stack()))
		}
	}()
	var (
		proto = protoSFlow
		flows []models.Flow
		err   error
	)
	if isSFlow {
		flows, err = s.dec.decodeSFlow(data, src, recv)
	} else {
		proto, flows, err = s.dec.decodeNetFlow(data, src, recv)
	}
	metrics.FlowDatagramsTotal.WithLabelValues(proto).Inc()
	if err != nil {
		reason := reasonMalformed
		var de *decodeError
		if errors.As(err, &de) {
			reason = de.reason
		}
		metrics.FlowDecodeErrors.WithLabelValues(proto, reason).Inc()
		slog.Debug("Flow ingest: decode error", "protocol", proto, "exporter", src.String(), "error", err)
	}
	if len(flows) == 0 {
		return
	}
	metrics.FlowRecordsDecoded.WithLabelValues(proto).Add(float64(len(flows)))
	for i := range flows {
		s.writer.enqueue(flows[i])
	}
}

func (s *Server) lookupDevice(a netip.Addr) (int64, bool) {
	m := s.devices.Load()
	if m == nil {
		return 0, false
	}
	id, ok := (*m)[a]
	return id, ok
}

func (s *Server) deviceRefreshLoop(ctx context.Context) {
	t := time.NewTicker(s.cfg.DeviceRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refreshDevices(ctx)
		}
	}
}

// refreshDevices reloads the exporter IP -> device ID map. On error the
// previous map is kept. When several devices share an IP the lowest ID wins.
func (s *Server) refreshDevices(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	devices, err := s.store.GetDevices(ctx)
	if err != nil {
		slog.Warn("Flow ingest: device refresh failed", "error", err)
		return
	}
	m := make(map[netip.Addr]int64, len(devices))
	for _, d := range devices {
		a, err := netip.ParseAddr(d.IPAddress)
		if err != nil {
			continue // hostnames are not matched
		}
		a = a.Unmap()
		if cur, ok := m[a]; !ok || d.ID < cur {
			m[a] = d.ID
		}
	}
	s.devices.Store(&m)
}
