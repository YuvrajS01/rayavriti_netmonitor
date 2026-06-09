package handlers

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type CaptureHandler struct {
	running int32
	db      database.Database
	hub     *websocket.Hub

	mu      sync.Mutex
	cancel  context.CancelFunc
	stats   captureStats
}

type captureStats struct {
	totalPackets int64
	totalBytes   int64
	protocols    map[string]int64
}

func NewCaptureHandler(db database.Database, hub *websocket.Hub) *CaptureHandler {
	return &CaptureHandler{db: db, hub: hub}
}

func (h *CaptureHandler) Start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Interface string `json:"interface"`
		Filter    string `json:"filter"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if body.Interface == "" {
		httputil.SendError(w, 400, "interface is required")
		return
	}

	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		httputil.SendError(w, 409, "capture already running")
		return
	}

	session := &models.CaptureSession{
		InterfaceName: body.Interface,
		Filter:        body.Filter,
		Status:        "running",
		TotalPackets:  0,
		TotalBytes:    0,
	}
	created, err := h.db.CreateCaptureSession(r.Context(), session)
	if err != nil {
		atomic.StoreInt32(&h.running, 0)
		httputil.SendError(w, 500, err.Error())
		return
	}

	h.mu.Lock()
	h.stats = captureStats{protocols: map[string]int64{}}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.mu.Unlock()

	go h.runCapture(ctx, created.ID, body.Interface, body.Filter)

	h.hub.Broadcast(websocket.Message{
		Type: websocket.EventCaptureStatus,
		Data: map[string]any{"sessionId": created.ID, "status": "running", "interface": body.Interface},
	})

	httputil.SendCreated(w, created)
}

func (h *CaptureHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}

	h.mu.Lock()
	cancel := h.cancel
	h.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	h.mu.Lock()
	stats := models.CaptureSessionStats{
		TotalPackets: h.stats.totalPackets,
		TotalBytes:   h.stats.totalBytes,
	}
	h.mu.Unlock()

	if err := h.db.StopCaptureSession(r.Context(), id, stats); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	atomic.StoreInt32(&h.running, 0)

	h.hub.Broadcast(websocket.Message{
		Type: websocket.EventCaptureStatus,
		Data: map[string]any{"sessionId": id, "status": "stopped"},
	})

	httputil.SendOK(w, map[string]any{"status": "stopped", "totalPackets": stats.TotalPackets, "totalBytes": stats.TotalBytes})
}

func (h *CaptureHandler) Stats(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	s := h.stats
	h.mu.Unlock()

	httputil.SendOK(w, map[string]any{
		"running":      atomic.LoadInt32(&h.running) == 1,
		"totalPackets": s.totalPackets,
		"totalBytes":   s.totalBytes,
		"protocols":    s.protocols,
	})
}

func (h *CaptureHandler) Interfaces(w http.ResponseWriter, r *http.Request) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to list interfaces")
		return
	}

	type ifaceInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Addresses   []string `json:"addresses"`
		Flags       []string `json:"flags"`
	}

	var result []ifaceInfo
	for _, iface := range netInterfaces {
		info := ifaceInfo{
			Name:        iface.Name,
			Description: iface.HardwareAddr.String(),
			Flags:       parseFlags(iface.Flags.String()),
		}
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				info.Addresses = append(info.Addresses, addr.String())
			}
		}
		if info.Addresses == nil {
			info.Addresses = []string{}
		}
		if info.Flags == nil {
			info.Flags = []string{}
		}
		result = append(result, info)
	}
	if result == nil {
		result = []ifaceInfo{}
	}
	httputil.SendOK(w, result)
}

func parseFlags(flags string) []string {
	if flags == "" {
		return []string{}
	}
	parts := strings.Split(flags, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (h *CaptureHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	session, err := h.db.GetCaptureSession(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "session not found")
		return
	}
	httputil.SendOK(w, session)
}

func (h *CaptureHandler) GetPackets(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 200
	}
	packets, err := h.db.GetCapturePackets(r.Context(), id, limit, offset)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, packets)
}

func (h *CaptureHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	sessions, err := h.db.GetCaptureSessions(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	httputil.SendOK(w, sessions)
}

// runCapture launches tcpdump and parses its output into packets.
func (h *CaptureHandler) runCapture(ctx context.Context, sessionID int64, iface, filter string) {
	args := []string{"-i", iface, "-nn", "-l", "-q"}
	if filter != "" {
		args = append(args, filter)
	}
	// -e adds link-level headers, -tttt adds human-readable timestamps
	args = append(args, "-e", "-tttt")

	cmd := exec.CommandContext(ctx, "tcpdump", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to start tcpdump", "error", err)
		h.stopSession(sessionID, "error", "failed to start tcpdump: "+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start tcpdump", "error", err)
		h.stopSession(sessionID, "error", "failed to start tcpdump: "+err.Error())
		return
	}

	slog.Info("tcpdump started", "interface", iface, "filter", filter, "pid", cmd.Process.Pid)

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	var batch []models.CapturePacket
	batchTimer := time.NewTimer(500 * time.Millisecond)
	defer batchTimer.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		packets := batch
		batch = nil

		// Store in DB (best effort)
		for _, p := range packets {
			p := p
			if err := h.db.InsertCapturePacket(ctx, sessionID, &p); err != nil {
				slog.Warn("failed to insert packet", "error", err)
			}
		}

		// Broadcast via WebSocket
		packetData := make([]map[string]any, 0, len(packets))
		for _, p := range packets {
			packetData = append(packetData, map[string]any{
				"timestamp": p.Timestamp,
				"srcIp":     p.SrcIP,
				"dstIp":     p.DstIP,
				"srcPort":   p.SrcPort,
				"dstPort":   p.DstPort,
				"protocol":  p.Protocol,
				"length":    p.Length,
				"flags":     p.Flags,
			})
		}
		h.hub.Broadcast(websocket.Message{
			Type: websocket.EventCapturePacket,
			Data: map[string]any{
				"sessionId": sessionID,
				"packets":   packetData,
				"count":     len(packetData),
			},
		})
	}

	for scanner.Scan() {
		line := scanner.Text()
		pkt := parseTcpdumpLine(line)
		if pkt == nil {
			continue
		}

		h.mu.Lock()
		h.stats.totalPackets++
		h.stats.totalBytes += int64(pkt.Length)
		h.stats.protocols[pkt.Protocol]++
		h.mu.Unlock()

		batch = append(batch, *pkt)

		select {
		case <-batchTimer.C:
			flushBatch()
			batchTimer.Reset(500 * time.Millisecond)
		default:
		}

		select {
		case <-ctx.Done():
			flushBatch()
			return
		default:
		}
	}

	flushBatch()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == nil {
			slog.Error("tcpdump exited with error", "error", err)
			h.stopSession(sessionID, "error", "tcpdump exited: "+err.Error())
			return
		}
	}

	if atomic.LoadInt32(&h.running) == 1 {
		h.stopSession(sessionID, "stopped", "")
	}
}

func (h *CaptureHandler) stopSession(sessionID int64, status, errMsg string) {
	h.mu.Lock()
	stats := models.CaptureSessionStats{
		TotalPackets: h.stats.totalPackets,
		TotalBytes:   h.stats.totalBytes,
		ErrorMessage: errMsg,
	}
	h.mu.Unlock()

	if err := h.db.StopCaptureSession(context.Background(), sessionID, stats); err != nil {
		slog.Error("failed to stop capture session in DB", "error", err)
	}
	atomic.StoreInt32(&h.running, 0)

	h.hub.Broadcast(websocket.Message{
		Type: websocket.EventCaptureStatus,
		Data: map[string]any{"sessionId": sessionID, "status": status},
	})
}

// parseTcpdumpLine parses a tcpdump -nn -l -e -tttt output line.
// Example: "06:28:34.123456 IntelCor_xxxx > Broadcast, IPv4 (len=66), ..."
func parseTcpdumpLine(line string) *models.CapturePacket {
	if line == "" || strings.HasPrefix(line, "tcpdump:") || strings.HasPrefix(line, "listening") {
		return nil
	}

	pkt := &models.CapturePacket{
		Timestamp: time.Now(),
		Protocol:  "unknown",
	}

	// Try to extract timestamp (first 15 chars: "MM/DD/YYYY HH:MM:SS")
	if len(line) > 15 {
		tsPart := strings.TrimSpace(line[:15])
		if t, err := time.Parse("01/02/2006 15:04:05", tsPart); err == nil {
			pkt.Timestamp = t
		}
	}

	lower := strings.ToLower(line)

	// Detect protocol from content
	switch {
	case strings.Contains(lower, "ipv6"):
		pkt.Protocol = "IPv6"
	case strings.Contains(lower, "arp"):
		pkt.Protocol = "ARP"
	case strings.Contains(lower, "icmp"):
		pkt.Protocol = "ICMP"
	case strings.Contains(lower, "tcp"):
		pkt.Protocol = "TCP"
	case strings.Contains(lower, "udp"):
		pkt.Protocol = "UDP"
	}

	// Extract length: look for "len=NNN" or "(len NNN)"
	if idx := strings.Index(line, "len="); idx >= 0 {
		rest := line[idx+4:]
		end := strings.IndexAny(rest, ",) \t")
		if end > 0 {
			if n, err := strconv.Atoi(rest[:end]); err == nil {
				pkt.Length = n
			}
		}
	} else if idx := strings.Index(line, "(len "); idx >= 0 {
		rest := line[idx+5:]
		end := strings.IndexAny(rest, ",) \t")
		if end > 0 {
			if n, err := strconv.Atoi(rest[:end]); err == nil {
				pkt.Length = n
			}
		}
	}
	if pkt.Length == 0 {
		pkt.Length = 64 // default
	}

	// Try to extract IP addresses: look for patterns like "A.B.C.D" or "A.B.C.D.port"
	// tcpdump -nn output has IPs like "192.168.1.1.443" or "10.0.0.1"
	words := strings.Fields(line)
	for _, w := range words {
		w = strings.Trim(w, ",:;()>[]")
		parts := strings.Split(w, ".")
		if len(parts) == 4 {
			// Check if all parts are numeric
			allNum := true
			for _, p := range parts {
				if _, err := strconv.Atoi(p); err != nil {
					allNum = false
					break
				}
			}
			if allNum {
				if pkt.SrcIP == "" {
					pkt.SrcIP = w
				} else if pkt.DstIP == "" {
					pkt.DstIP = w
				}
			}
		}
	}

	// If we couldn't parse IPs from the line, use placeholder
	if pkt.SrcIP == "" {
		pkt.SrcIP = "unknown"
	}
	if pkt.DstIP == "" {
		pkt.DstIP = "unknown"
	}

	// Extract port from "port.NNN" pattern (tcpdump -nn output)
	if idx := strings.Index(lower, ".port"); idx >= 0 {
		rest := line[idx+5:]
		end := strings.IndexAny(rest, ",) \t")
		if end > 0 {
			if n, err := strconv.Atoi(rest[:end]); err == nil && n > 0 && n < 65536 {
				if pkt.SrcPort == 0 {
					pkt.SrcPort = n
				} else {
					pkt.DstPort = n
				}
			}
		}
	}

	// Extract flags
	if idx := strings.Index(lower, "flags ["); idx >= 0 {
		rest := line[idx+7:]
		end := strings.IndexByte(rest, ']')
		if end > 0 {
			pkt.Flags = strings.TrimSpace(rest[:end])
		}
	} else if idx := strings.Index(lower, "flags "); idx >= 0 {
		rest := line[idx+6:]
		end := strings.IndexAny(rest, ",) \t")
		if end > 0 {
			pkt.Flags = strings.TrimSpace(rest[:end])
		}
	}

	return pkt
}
