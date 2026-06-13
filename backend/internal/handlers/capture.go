package handlers

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"regexp"
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

// bpfFilterRegex allows only valid BPF filter characters.
var bpfFilterRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\.\-\+\*\/\<\>\=&\|!()]+$`)

type CaptureHandler struct {
	running int32
	db      database.Database
	hub     *websocket.Hub

	mu     sync.Mutex
	cancel context.CancelFunc
	stats  captureStats
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

	// Validate interface exists on the system
	if !isValidInterface(body.Interface) {
		httputil.SendError(w, 400, "invalid network interface")
		return
	}

	// Validate BPF filter to prevent command injection
	if body.Filter != "" && !isValidBPFFilter(body.Filter) {
		httputil.SendError(w, 400, "invalid capture filter")
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
	if limit < len(sessions) {
		sessions = sessions[:limit]
	}
	httputil.SendOK(w, sessions)
}

// runCapture launches tcpdump and parses its output into packets.
func (h *CaptureHandler) runCapture(ctx context.Context, sessionID int64, iface, filter string) {
	args := []string{"-i", iface, "-nn", "-l", "-x"}
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
			_ = cmd.Process.Kill()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	var batch []models.CapturePacket
	var currentPkt *models.CapturePacket
	var hexLines []string
	batchTimer := time.NewTimer(500 * time.Millisecond)
	defer batchTimer.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		packets := batch
		batch = nil

		// Store in DB (best effort)
		for _, pkt := range packets {
			if err := h.db.InsertCapturePacket(ctx, sessionID, &pkt); err != nil {
				slog.Warn("failed to insert packet", "error", err)
			}
		}

		// Broadcast via WebSocket
		packetData := make([]map[string]any, 0, len(packets))
		for _, pkt := range packets {
			packetData = append(packetData, map[string]any{
				"timestamp": pkt.Timestamp,
				"srcIp":     pkt.SrcIP,
				"dstIp":     pkt.DstIP,
				"srcPort":   pkt.SrcPort,
				"dstPort":   pkt.DstPort,
				"protocol":  pkt.Protocol,
				"length":    pkt.Length,
				"flags":     pkt.Flags,
				"payload":   pkt.Payload,
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

	// finalizePacket attaches accumulated hex data and adds the packet to the batch.
	finalizePacket := func() {
		if currentPkt != nil {
			if len(hexLines) > 0 {
				currentPkt.Payload = strings.Join(hexLines, " ")
			}
			h.mu.Lock()
			h.stats.totalPackets++
			h.stats.totalBytes += int64(currentPkt.Length)
			h.stats.protocols[currentPkt.Protocol]++
			h.mu.Unlock()

			batch = append(batch, *currentPkt)
			currentPkt = nil
			hexLines = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if isTcpdumpHeader(line) {
			// New packet header: finalize previous packet
			finalizePacket()

			pkt := parseTcpdumpHeader(line)
			if pkt == nil {
				continue
			}
			currentPkt = pkt
		} else if currentPkt != nil && strings.HasPrefix(strings.TrimSpace(line), "0x") {
			// Hex dump line: extract hex bytes
			trimmed := strings.TrimSpace(line)
			// Format: "0x0000:  4500 003c 1234 4000 4006 a1b2 c0a8 0101"
			if colonIdx := strings.Index(trimmed, ":"); colonIdx >= 0 {
				hexPart := trimmed[colonIdx+1:]
				hexPart = strings.TrimSpace(hexPart)
				if hexPart != "" {
					hexLines = append(hexLines, hexPart)
				}
			}
		}

		select {
		case <-batchTimer.C:
			finalizePacket()
			flushBatch()
			batchTimer.Reset(500 * time.Millisecond)
		default:
		}

		select {
		case <-ctx.Done():
			finalizePacket()
			flushBatch()
			return
		default:
		}
	}

	finalizePacket()
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

// isTcpdumpHeader returns true if the line is a packet header (starts with timestamp).
func isTcpdumpHeader(line string) bool {
	if len(line) < 26 {
		return false
	}
	// With -tttt, timestamps look like "2026-06-11 14:23:37.123456"
	if line[4] != '-' || line[7] != '-' || line[10] != ' ' || line[13] != ':' || line[16] != ':' {
		return false
	}
	return true
}

// parseTcpdumpHeader parses a tcpdump -nn -l -e -tttt -x output line (the header line).
// Example: "2026-06-11 14:23:37.123456 eth0 In  IP 192.168.1.1.443 > 10.0.0.1.8080: Flags [S], ..."
func parseTcpdumpHeader(line string) *models.CapturePacket {
	if line == "" || strings.HasPrefix(line, "tcpdump:") || strings.HasPrefix(line, "listening on") {
		return nil
	}

	pkt := &models.CapturePacket{
		Timestamp: time.Now(),
		Protocol:  "unknown",
	}

	// Parse timestamp: "YYYY-MM-DD HH:MM:SS.micro"
	if len(line) >= 26 {
		tsPart := line[:26]
		if t, err := time.Parse("2006-01-02 15:04:05.000000", tsPart); err == nil {
			pkt.Timestamp = t
		}
	}

	lower := strings.ToLower(line)

	// Detect protocol from content
	// With -e, output includes "ethertype IPv4/IPv6/ARP (0xNNNN)"
	// TCP packets show "Flags [S/.//P/F]" but may not contain the word "tcp"
	switch {
	case strings.Contains(lower, "arp"):
		pkt.Protocol = "ARP"
	case strings.Contains(lower, "icmp6") || (strings.Contains(lower, "ipv6") && strings.Contains(lower, "icmp")):
		pkt.Protocol = "ICMP6"
	case strings.Contains(lower, "icmp"):
		pkt.Protocol = "ICMP"
	case strings.Contains(lower, "udp"):
		pkt.Protocol = "UDP"
	case strings.Contains(lower, "flags [") || strings.Contains(lower, "flags "):
		pkt.Protocol = "TCP"
	case strings.Contains(lower, "tcp"):
		pkt.Protocol = "TCP"
	case strings.Contains(lower, "ipv4") || strings.Contains(lower, "ipv6"):
		pkt.Protocol = "TCP" // Default IP traffic to TCP
	}

	// Extract length: look for "length NNN" or "len=NNN"
	if idx := strings.Index(line, "length "); idx >= 0 {
		rest := line[idx+7:]
		end := strings.IndexAny(rest, ",)\n\t")
		if end > 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(rest[:end])); err == nil {
				pkt.Length = n
			}
		}
	} else if idx := strings.Index(line, "len="); idx >= 0 {
		rest := line[idx+4:]
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

	// Extract IP addresses: tcpdump -nn outputs "SRC_IP > DST_IP:"
	// With -e, there are two ">" characters: first between MACs, then between IPs.
	// We need the IP-level ">", which appears after "length N:".
	// For ARP, format is "who-has IP1 tell IP2" or "Reply IP is-at MAC".
	if pkt.Protocol == "ARP" {
		// ARP request: "who-has 192.168.0.115 tell 192.168.0.1"
		if idx := strings.Index(lower, "who-has "); idx >= 0 {
			rest := line[idx+8:]
			end := strings.IndexAny(rest, ", \t")
			if end > 0 {
				pkt.DstIP = strings.TrimSpace(rest[:end])
			}
		}
		if idx := strings.Index(lower, " tell "); idx >= 0 {
			rest := line[idx+6:]
			end := strings.IndexAny(rest, ", \t")
			if end > 0 {
				pkt.SrcIP = strings.TrimSpace(rest[:end])
			}
		}
		// ARP reply: "Reply 192.168.0.1 is-at 00:11:22:33:44:55"
		if idx := strings.Index(lower, "reply "); idx >= 0 {
			rest := line[idx+6:]
			end := strings.IndexAny(rest, ", \t")
			if end > 0 {
				pkt.SrcIP = strings.TrimSpace(rest[:end])
			}
		}
		// Fallback: if we still don't have IPs, try to extract any IPv4 from the line
		if pkt.SrcIP == "" || pkt.DstIP == "" {
			words := strings.Fields(line)
			for _, w := range words {
				w = strings.Trim(w, ",:;()>[]")
				ip, _ := splitIPPortV2(w)
				if ip != "" && isValidIPv4(ip) {
					if pkt.SrcIP == "" {
						pkt.SrcIP = ip
					} else if pkt.DstIP == "" && ip != pkt.SrcIP {
						pkt.DstIP = ip
						break
					}
				}
			}
		}
	} else {
		// IP/IPv6/TCP/UDP: find the ">" that appears after "length N:"
		// Format: "... length 66: SRC > DST: Flags ..."
		arrowIdx := -1
		if lenIdx := strings.Index(line, "length "); lenIdx >= 0 {
			// Find ">" after the "length N:" part
			colonAfterLen := strings.Index(line[lenIdx:], ":")
			if colonAfterLen >= 0 {
				searchFrom := lenIdx + colonAfterLen + 1
				if idx := strings.Index(line[searchFrom:], " > "); idx >= 0 {
					arrowIdx = searchFrom + idx
				}
			}
		}
		if arrowIdx >= 0 {
			// Extract source and destination from around the " > "
			srcPart := strings.TrimSpace(line[:arrowIdx])
			dstPart := strings.TrimSpace(line[arrowIdx+3:])

			// Source: take last whitespace-delimited token
			srcWords := strings.Fields(srcPart)
			if len(srcWords) > 0 {
				srcRaw := strings.Trim(srcWords[len(srcWords)-1], ",:;()>[]")
				ip, port := splitIPPortV2(srcRaw)
				if ip != "" {
					pkt.SrcIP = ip
					pkt.SrcPort = port
				}
			}

			// Destination: take first token before ":" or ","
			dstEnd := strings.IndexAny(dstPart, ":, \t")
			if dstEnd > 0 {
				dstRaw := strings.Trim(dstPart[:dstEnd], ",:;()>[]")
				ip, port := splitIPPortV2(dstRaw)
				if ip != "" {
					pkt.DstIP = ip
					pkt.DstPort = port
				}
			}
		}
	}

	if pkt.SrcIP == "" {
		pkt.SrcIP = "unknown"
	}
	if pkt.DstIP == "" {
		pkt.DstIP = "unknown"
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

// splitIPPortV2 splits "192.168.1.1.443" into ("192.168.1.1", 443),
// "192.168.1.1" into ("192.168.1.1", 0), or
// "[fe80::1]:443" into ("fe80::1", 443), or
// "fe80::1" into ("fe80::1", 0).
func splitIPPortV2(s string) (string, int) {
	// Handle bracketed IPv6: "[fe80::1]:443"
	if strings.HasPrefix(s, "[") {
		if idx := strings.Index(s, "]"); idx > 0 {
			ip := s[1:idx]
			rest := s[idx+1:]
			if strings.HasPrefix(rest, ":") {
				if port, err := strconv.Atoi(rest[1:]); err == nil && port > 0 && port < 65536 {
					return ip, port
				}
			}
			return ip, 0
		}
	}

	// IPv6 contains colons — check if it's a bare IPv6 address
	if strings.Count(s, ":") >= 2 {
		// Could be IPv6 with port: "fe80::1.443" won't happen, but plain IPv6 is fine
		// Strip trailing port-like segment only if first parts are valid hex
		if isValidIPv6(s) {
			return s, 0
		}
		// Try removing last colon-separated segment as port
		if idx := strings.LastIndex(s, ":"); idx > 0 {
			ip := s[:idx]
			portStr := s[idx+1:]
			if isValidIPv6(ip) {
				if port, err := strconv.Atoi(portStr); err == nil && port > 0 && port < 65536 {
					return ip, port
				}
			}
		}
	}

	// IPv4: "192.168.1.1.443" (5 dots) or "192.168.1.1" (4 dots)
	parts := strings.Split(s, ".")
	if len(parts) == 5 {
		ip := strings.Join(parts[:4], ".")
		if isValidIPv4(ip) {
			if port, err := strconv.Atoi(parts[4]); err == nil && port > 0 && port < 65536 {
				return ip, port
			}
			return ip, 0
		}
	} else if len(parts) == 4 {
		if isValidIPv4(s) {
			return s, 0
		}
	}
	return "", 0
}

// isValidIPv4 checks if a string is a valid dotted-decimal IPv4 address.
func isValidIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

// isValidIPv6 checks if a string looks like an IPv6 address (contains 2+ colons).
func isValidIPv6(s string) bool {
	if strings.Count(s, ":") < 2 {
		return false
	}
	// Basic check: no dots, all hex chars and colons
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == ':') {
			return false
		}
	}
	return true
}

// isValidInterface checks if the given interface name exists on the system.
func isValidInterface(name string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}

// isValidBPFFilter validates that a BPF filter contains only safe characters.
func isValidBPFFilter(filter string) bool {
	if len(filter) > 512 {
		return false
	}
	return bpfFilterRegex.MatchString(filter)
}
