package discovery

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultConcurrency = 64
	pingTimeoutSec     = 2
	tcpTimeout         = 2 * time.Second
	arpRefreshDelay    = 500 * time.Millisecond
)

var commonPorts = []int{21, 22, 23, 25, 53, 80, 161, 443, 515, 554, 631, 9100, 8080, 8443, 2000, 5060, 1900}

var ouiTable = map[string]string{
	"00:01:42": "Cisco",
	"00:03:6B": "Cisco",
	"00:04:96": "Cisco",
	"00:0A:41": "Cisco",
	"00:0B:BE": "Cisco",
	"00:0C:85": "Cisco",
	"00:0E:38": "Cisco",
	"00:0E:D7": "Cisco",
	"00:11:21": "Cisco",
	"00:12:79": "Cisco",
	"00:13:7F": "Cisco",
	"00:14:6A": "Cisco",
	"00:16:47": "Cisco",
	"00:17:0E": "Cisco",
	"00:17:94": "Cisco",
	"00:18:73": "Cisco",
	"00:19:AA": "Cisco",
	"00:1A:2B": "Cisco",
	"00:1A:A1": "Cisco",
	"00:1B:0D": "Cisco",
	"00:1B:53": "Cisco",
	"00:1C:0E": "Cisco",
	"00:1C:0F": "Cisco",
	"00:1C:10": "Cisco",
	"00:1C:11": "Cisco",
	"00:1E:13": "Cisco",
	"00:1E:49": "Cisco",
	"00:21:55": "Cisco",
	"00:22:55": "Cisco",
	"00:23:04": "Cisco",
	"00:23:33": "Cisco",
	"00:23:5D": "Cisco",
	"00:24:13": "Cisco",
	"00:24:50": "Cisco",
	"00:25:45": "Cisco",
	"00:25:84": "Cisco",
	"00:26:0A": "Cisco",
	"00:26:51": "Cisco",
	"00:26:98": "Cisco",
	"00:26:AB": "Cisco",
	"00:27:0C": "Cisco",
	"00:27:23": "Cisco",
	"40:A6:77": "Cisco",
	"58:6D:8F": "Cisco",
	"68:86:A7": "Cisco",
	"6C:9C:BF": "Cisco",
	"70:DB:98": "Cisco",
	"84:78:AC": "Cisco",
	"84:B5:9C": "Cisco",
	"88:43:DD": "Cisco",
	"90:21:06": "Cisco",
	"90:84:0D": "Cisco",
	"94:10:3E": "Cisco",
	"94:57:A5": "Cisco",
	"A0:3D:6F": "Cisco",
	"A0:57:E3": "Cisco",
	"A0:EC:F9": "Cisco",
	"A4:07:B6": "Cisco",
	"A4:56:30": "Cisco",
	"A8:0C:63": "Cisco",
	"A8:9D:C1": "Cisco",
	"B0:72:BF": "Cisco",
	"B4:A4:E3": "Cisco",
	"B4:B0:24": "Cisco",
	"B8:38:61": "Cisco",
	"B8:BE:BF": "Cisco",
	"C0:25:E9": "Cisco",
	"C0:61:18": "Cisco",
	"C0:70:09": "Cisco",
	"C0:7B:BC": "Cisco",
	"C4:64:13": "Cisco",
	"C4:71:54": "Cisco",
	"C4:B2:39": "Cisco",
	"C8:00:84": "Cisco",
	"C8:B7:9D": "Cisco",
	"CC:16:7E": "Cisco",
	"CC:46:D6": "Cisco",
	"CC:5A:9A": "Cisco",
	"CC:EF:48": "Cisco",
	"D0:21:F9": "Cisco",
	"D0:72:DC": "Cisco",
	"D0:73:D5": "Cisco",
	"D0:B0:CD": "Cisco",
	"D4:D7:48": "Cisco",
	"D8:24:BD": "Cisco",
	"D8:B1:90": "Cisco",
	"DC:08:56": "Cisco",
	"DC:57:19": "Cisco",
	"DC:77:5C": "Cisco",
	"DC:CB:BB": "Cisco",
	"E0:2F:46": "Cisco",
	"E0:5B:39": "Cisco",
	"E0:5F:45": "Cisco",
	"E0:60:66": "Cisco",
	"E0:EC:77": "Cisco",
	"E4:AA:5D": "Cisco",
	"E4:D3:32": "Cisco",
	"E8:04:62": "Cisco",
	"E8:65:49": "Cisco",
	"E8:BD:1D": "Cisco",
	"EC:44:76": "Cisco",
	"EC:CE:13": "Cisco",
	"F0:1C:2D": "Cisco",
	"F0:29:29": "Cisco",
	"F0:72:EA": "Cisco",
	"F0:B4:79": "Cisco",
	"F0:F7:55": "Cisco",
	"F4:4E:05": "Cisco",
	"F4:CF:E2": "Cisco",
	"F8:4A:13": "Cisco",
	"F8:72:5C": "Cisco",
	"FC:5B:39": "Cisco",
	"FC:99:47": "Cisco",
	"00:0C:29": "VMware",
	"00:50:56": "VMware",
	"00:05:69": "VMware",
	"00:1C:14": "VMware",
	"00:50:F2": "Microsoft",
	"00:03:FF": "Microsoft",
	"00:15:5D": "Microsoft",
	"7C:1E:52": "Microsoft",
	"00:08:74": "Dell",
	"00:0B:DB": "Dell",
	"00:0D:56": "Dell",
	"00:0F:1F": "Dell",
	"00:11:43": "Dell",
	"00:12:3F": "Dell",
	"00:13:72": "Dell",
	"00:14:22": "Dell",
	"00:15:C5": "Dell",
	"00:18:8B": "Dell",
	"00:19:B9": "Dell",
	"00:1B:B9": "Dell",
	"00:1C:43": "Dell",
	"00:1D:09": "Dell",
	"00:1E:65": "Dell",
	"00:1E:67": "Dell",
	"00:1E:68": "Dell",
	"00:1F:29": "Dell",
	"00:21:70": "Dell",
	"00:21:9B": "Dell",
	"00:22:19": "Dell",
	"00:22:48": "Dell",
	"00:24:D7": "Dell",
	"00:25:64": "Dell",
	"00:25:90": "Dell",
	"00:25:AB": "Dell",
	"00:26:5A": "Dell",
	"00:26:B9": "Dell",
	"00:27:0E": "Dell",
	"18:03:73": "Dell",
	"18:66:DA": "Dell",
	"18:A9:05": "Dell",
	"18:DB:72": "Dell",
	"24:6E:96": "Dell",
	"28:F1:0E": "Dell",
	"2C:BE:08": "Dell",
	"30:D0:42": "Dell",
	"40:9F:38": "Dell",
	"44:AF:28": "Dell",
	"48:4D:7E": "Dell",
	"50:9A:4C": "Dell",
	"54:BF:64": "Dell",
	"58:82:A8": "Dell",
	"5C:26:0A": "Dell",
	"60:18:95": "Dell",
	"64:00:6A": "Dell",
	"68:4F:64": "Dell",
	"70:9C:A6": "Dell",
	"74:86:C2": "Dell",
	"78:2B:CB": "Dell",
	"78:E7:D1": "Dell",
	"7C:2B:91": "Dell",
	"80:18:44": "Dell",
	"80:90:6C": "Dell",
	"84:7B:EB": "Dell",
	"84:8F:69": "Dell",
	"88:6F:D1": "Dell",
	"90:B1:1C": "Dell",
	"94:65:9C": "Dell",
	"98:90:96": "Dell",
	"9C:EB:E8": "Dell",
	"A0:48:1C": "Dell",
	"A4:1F:72": "Dell",
	"A4:BB:6D": "Dell",
	"B0:46:FC": "Dell",
	"B0:83:FE": "Dell",
	"B4:9D:BF": "Dell",
	"B8:2A:72": "Dell",
	"B8:CA:3A": "Dell",
	"B8:EB:1F": "Dell",
	"BC:30:5B": "Dell",
	"BC:76:70": "Dell",
	"BC:A4:F1": "Dell",
	"C0:3F:0E": "Dell",
	"C8:1F:66": "Dell",
	"CC:3E:5F": "Dell",
	"CC:52:AF": "Dell",
	"D0:94:66": "Dell",
	"D0:DB:32": "Dell",
	"D4:AE:52": "Dell",
	"D4:BF:80": "Dell",
	"D8:CC:89": "Dell",
	"DC:3B:DB": "Dell",
	"E0:DB:10": "Dell",
	"E4:F0:04": "Dell",
	"E8:6C:12": "Dell",
	"EC:F4:BB": "Dell",
	"F0:1F:AF": "Dell",
	"F0:BD:31": "Dell",
	"F4:8E:38": "Dell",
	"F8:BC:12": "Dell",
	"FC:15:B4": "Dell",
	"FC:51:A4": "Dell",
	"00:0E:7F": "HP",
	"00:10:83": "HP",
	"00:10:B5": "HP",
	"00:11:0A": "HP",
	"00:11:85": "HP",
	"00:13:21": "HP",
	"00:14:38": "HP",
	"00:15:60": "HP",
	"00:16:35": "HP",
	"00:17:A4": "HP",
	"00:18:FE": "HP",
	"00:19:BB": "HP",
	"00:1A:4B": "HP",
	"00:1B:78": "HP",
	"00:1C:C4": "HP",
	"00:1D:28": "HP",
	"00:1E:0B": "HP",
	"00:1E:41": "HP",
	"00:21:5A": "HP",
	"00:22:64": "HP",
	"00:25:B3": "HP",
	"00:26:BB": "HP",
	"00:30:C1": "HP",
	"00:50:B6": "HP",
	"00:80:E0": "HP",
	"04:EA:56": "HP",
	"08:2E:5F": "HP",
	"0C:75:BD": "HP",
	"0C:C4:7A": "HP",
	"10:1F:74": "HP",
	"10:60:4B": "HP",
	"14:02:EC": "HP",
	"14:58:D2": "HP",
	"18:A9:9B": "HP",
	"18:E2:88": "HP",
	"1C:3B:2E": "HP",
	"20:25:D0": "HP",
	"24:B7:2A": "HP",
	"28:92:4A": "HP",
	"2C:27:D7": "HP",
	"2C:41:38": "HP",
	"30:E1:71": "HP",
	"38:63:BB": "HP",
	"38:F7:3D": "HP",
	"3C:4A:92": "HP",
	"40:B0:34": "HP",
	"40:E7:30": "HP",
	"44:31:32": "HP",
	"48:0F:CF": "HP",
	"4C:34:88": "HP",
	"50:65:F3": "HP",
	"50:EB:71": "HP",
	"54:04:A6": "HP",
	"58:0A:E6": "HP",
	"5C:B9:01": "HP",
	"60:C5:47": "HP",
	"64:51:06": "HP",
	"64:80:99": "HP",
	"6C:3B:6B": "HP",
	"74:46:A0": "HP",
	"78:AC:C0": "HP",
	"7C:2F:67": "HP",
	"80:27:6C": "HP",
	"80:71:1F": "HP",
	"84:34:97": "HP",
	"88:51:FB": "HP",
	"8C:3B:AD": "HP",
	"90:90:E0": "HP",
	"98:E7:F4": "HP",
	"9C:32:CE": "HP",
	"A0:1D:48": "HP",
	"A4:5D:36": "HP",
	"A8:23:FE": "HP",
	"AC:16:2D": "HP",
	"B4:39:D6": "HP",
	"B4:99:BA": "HP",
	"B4:B5:2F": "HP",
	"B8:CB:EC": "HP",
	"C0:91:34": "HP",
	"C8:CB:B8": "HP",
	"D0:7E:28": "HP",
	"D4:3D:7E": "HP",
	"D8:D3:85": "HP",
	"DC:4A:3E": "HP",
	"E4:11:5B": "HP",
	"E8:F7:24": "HP",
	"EC:88:92": "HP",
	"F0:92:1C": "HP",
	"F4:39:09": "HP",
	"F8:0F:41": "HP",
	"00:04:5A": "Linksys",
	"00:1A:70": "Linksys",
	"00:21:29": "Linksys",
	"00:23:69": "Linksys",
	"00:25:9C": "Linksys",
	"04:9F:81": "Linksys",
	"08:37:3A": "Linksys",
	"10:BF:48": "Linksys",
	"14:CF:92": "Linksys",
	"18:E8:29": "Linksys",
	"20:AA:4B": "Linksys",
	"28:0C:2D": "Linksys",
	"2C:10:5B": "Linksys",
	"34:57:60": "Linksys",
	"40:16:9E": "Linksys",
	"44:23:7C": "Linksys",
	"48:5B:39": "Linksys",
	"54:53:ED": "Linksys",
	"60:38:E0": "Linksys",
	"64:BC:0C": "Linksys",
	"68:72:51": "Linksys",
	"6C:50:4D": "Linksys",
	"70:65:8B": "Linksys",
	"74:4D:28": "Linksys",
	"7C:8B:CA": "Linksys",
	"84:1B:5E": "Linksys",
	"88:43:E1": "Linksys",
	"8C:21:0A": "Linksys",
	"90:72:40": "Linksys",
	"98:FC:11": "Linksys",
	"A0:04:60": "Linksys",
	"A0:63:91": "Linksys",
	"A8:8C:2D": "Linksys",
	"AC:22:05": "Linksys",
	"B0:26:80": "Linksys",
	"B0:4E:26": "Linksys",
	"B4:55:70": "Linksys",
	"B8:EE:65": "Linksys",
	"BC:EE:7B": "Linksys",
	"C0:56:27": "Linksys",
	"C0:C1:C0": "Linksys",
	"C4:D9:87": "Linksys",
	"C8:3A:35": "Linksys",
	"D4:5D:64": "Linksys",
	"D8:31:CF": "Linksys",
	"DC:9F:DB": "Linksys",
	"E4:55:28": "Linksys",
	"E8:9F:80": "Linksys",
	"EC:1A:59": "Linksys",
	"F0:9F:C2": "Linksys",
	"F4:7B:5E": "Linksys",
	"F8:28:19": "Linksys",
	"08:00:27": "VirtualBox",
	"52:54:00": "QEMU/KVM",
	"00:16:3E": "Xen",
	"00:1B:21": "Intel",
	"00:08:C7": "Intel",
	"00:0E:8C": "Intel",
	"00:11:75": "Intel",
	"00:12:F0": "Intel",
	"00:13:02": "Intel",
	"00:13:E8": "Intel",
	"00:15:17": "Intel",
	"00:16:6F": "Intel",
	"00:17:F2": "Intel",
	"00:18:DE": "Intel",
	"00:19:D1": "Intel",
	"00:1C:C0": "Intel",
	"00:1D:E0": "Intel",
	"00:22:FA": "Intel",
	"00:23:14": "Intel",
	"00:24:D6": "Intel",
	"00:26:C6": "Intel",
	"3C:97:0E": "Intel",
	"44:85:00": "Intel",
	"48:51:B5": "Intel",
	"50:7B:9D": "Intel",
	"54:A0:50": "Intel",
	"58:96:1D": "Intel",
	"5C:51:4F": "Intel",
	"60:36:DD": "Intel",
	"60:6C:66": "Intel",
	"68:05:CA": "Intel",
	"68:17:29": "Intel",
	"6C:88:14": "Intel",
	"70:85:C2": "Intel",
	"74:40:BE": "Intel",
	"78:2B:46": "Intel",
	"78:59:5E": "Intel",
	"80:86:F2": "Intel",
	"84:3A:4B": "Intel",
	"88:70:6C": "Intel",
	"8C:EC:4B": "Intel",
	"90:61:AE": "Intel",
	"94:35:0A": "Intel",
	"A0:36:9F": "Intel",
	"A0:99:9B": "Intel",
	"A4:34:D9": "Intel",
	"A4:C4:94": "Intel",
	"A8:60:B6": "Intel",
	"AC:5F:3E": "Intel",
	"AC:87:A3": "Intel",
	"B0:A4:60": "Intel",
	"B0:6B:BF": "Intel",
	"B4:69:1F": "Intel",
	"B8:08:CF": "Intel",
	"B8:83:8F": "Intel",
	"BC:77:37": "Intel",
	"BC:E7:12": "Intel",
	"C0:41:7A": "Intel",
	"C4:6E:1F": "Intel",
	"C4:D3:D3": "Intel",
	"C8:08:29": "Intel",
	"C8:5B:76": "Intel",
	"C8:ED:B4": "Intel",
	"CC:D2:81": "Intel",
	"D0:03:4B": "Intel",
	"D0:4E:50": "Intel",
	"D8:9B:3B": "Intel",
	"DC:53:60": "Intel",
	"E0:6F:95": "Intel",
	"E4:5E:1A": "Intel",
	"E4:A7:A0": "Intel",
	"E4:B9:7A": "Intel",
	"E8:2A:44": "Intel",
	"E8:B1:FC": "Intel",
	"EC:15:6B": "Intel",
	"F0:D1:79": "Intel",
	"F0:DB:F2": "Intel",
	"F8:63:3F": "Intel",
	"FC:44:82": "Intel",
	"00:1B:54": "Juniper",
	"00:17:2A": "Juniper",
	"00:19:E2": "Juniper",
	"00:21:59": "Juniper",
	"00:22:83": "Juniper",
	"00:23:9C": "Juniper",
	"00:24:DC": "Juniper",
	"00:26:88": "Juniper",
	"28:8A:1C": "Juniper",
	"2C:6B:F5": "Juniper",
	"3C:61:04": "Juniper",
	"44:F4:77": "Juniper",
	"48:C5:CB": "Juniper",
	"4C:96:14": "Juniper",
	"50:C2:ED": "Juniper",
	"54:1E:56": "Juniper",
	"58:00:BB": "Juniper",
	"5C:45:27": "Juniper",
	"64:87:88": "Juniper",
	"68:E8:95": "Juniper",
	"6C:F3:7F": "Juniper",
	"70:83:8D": "Juniper",
	"74:88:8A": "Juniper",
	"78:FE:3D": "Juniper",
	"7C:2F:80": "Juniper",
	"84:18:88": "Juniper",
	"88:E0:F3": "Juniper",
	"8C:85:90": "Juniper",
	"90:69:AF": "Juniper",
	"94:B9:7E": "Juniper",
	"98:FE:94": "Juniper",
	"9C:CC:83": "Juniper",
	"A0:A1:30": "Juniper",
	"A8:D0:E5": "Juniper",
	"A8:D9:96": "Juniper",
	"AC:4B:9E": "Juniper",
	"B0:A8:6E": "Juniper",
	"B0:C6:9A": "Juniper",
	"B0:F9:63": "Juniper",
	"B4:FB:E4": "Juniper",
	"B8:69:F4": "Juniper",
	"B8:76:3F": "Juniper",
	"B8:81:98": "Juniper",
	"B8:C1:11": "Juniper",
	"B8:C7:1A": "Juniper",
	"B8:FC:B7": "Juniper",
	"BC:14:85": "Juniper",
	"BC:34:00": "Juniper",
	"C0:1E:9B": "Juniper",
	"C0:87:5A": "Juniper",
	"C0:C5:22": "Juniper",
	"C4:83:72": "Juniper",
	"C4:E9:2F": "Juniper",
	"C8:0E:93": "Juniper",
	"C8:6C:87": "Juniper",
	"CC:E1:7F": "Juniper",
	"D0:54:2B": "Juniper",
	"D4:04:FF": "Juniper",
	"DC:38:E1": "Juniper",
	"DC:45:46": "Juniper",
	"E0:0C:7F": "Juniper",
	"E0:1E:B9": "Juniper",
	"E0:37:17": "Juniper",
	"E0:97:96": "Juniper",
	"E4:1D:2D": "Juniper",
	"E4:37:55": "Juniper",
	"E4:8D:8C": "Juniper",
	"E8:06:88": "Juniper",
	"E8:38:A0": "Juniper",
	"E8:B3:E7": "Juniper",
	"E8:C0:EB": "Juniper",
	"EC:13:DB": "Juniper",
	"EC:3E:F7": "Juniper",
	"F0:1C:3D": "Juniper",
	"F0:1E:62": "Juniper",
	"F0:A9:68": "Juniper",
	"F0:D5:F9": "Juniper",
	"F4:A8:9D": "Juniper",
	"F4:B5:2F": "Juniper",
	"F4:B7:E2": "Juniper",
	"F4:C7:14": "Juniper",
	"F4:E2:C6": "Juniper",
	"F8:01:13": "Juniper",
	"F8:04:2E": "Juniper",
	"F8:C0:01": "Juniper",
	"F8:C0:B6": "Juniper",
	"FC:01:7C": "Juniper",
	"FC:BD:90": "Juniper",
	"FC:DB:3C": "Juniper",
	"FC:FC:E2": "Juniper",
	"30:23:03": "Belkin",
	"B8:27:EB": "Raspberry Pi",
}

type Scanner struct {
	pool *pgxpool.Pool
}

func NewScanner(pool *pgxpool.Pool) *Scanner {
	return &Scanner{pool: pool}
}

func (s *Scanner) Scan(ctx context.Context, jobID int64, subnet string, scanType string, locationID *int64, excludeKnown bool) error {
	s.updateJobStatus(ctx, jobID, "running", "")
	ips, err := expandCIDR(subnet)
	if err != nil {
		s.updateJobStatus(ctx, jobID, "failed", err.Error())
		return err
	}
	if excludeKnown {
		known, err := s.getKnownIPs(ctx)
		if err != nil {
			s.updateJobStatus(ctx, jobID, "failed", err.Error())
			return err
		}
		ips = excludeIPs(ips, known)
	}
	totalScanned := len(ips)
	_, err = s.pool.Exec(ctx,
		`UPDATE discovery_jobs SET total_ips_scanned = $1 WHERE id = $2`,
		totalScanned, jobID)
	if err != nil {
		s.updateJobStatus(ctx, jobID, "failed", err.Error())
		return err
	}
	refreshARPTable()
	type ipResult struct {
		result *DiscoveryResult
	}
	results := make([]ipResult, len(ips))
	var wg sync.WaitGroup
	sem := make(chan struct{}, defaultConcurrency)
	var devicesFound atomic.Int64
	for i, ip := range ips {
		select {
		case <-ctx.Done():
			s.updateJobStatus(ctx, jobID, "cancelled", "scan cancelled")
			return ctx.Err()
		default:
		}
		wg.Add(1)
		go func(idx int, ipAddr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				return
			default:
			}
			alive, respMs := pingHost(ctx, ipAddr)
			if !alive {
				return
			}
			mac := lookupARP(ipAddr)
			manufacturer := ""
			if mac != "" {
				manufacturer = ouiLookup(mac)
			}
			var openPorts []int
			if scanType == "full" || scanType == "ping_snmp" {
				openPorts = probeTCPPorts(ctx, ipAddr, commonPorts)
			}
			category := guessCategoryFromPorts(openPorts)
			hostname := reverseLookup(ipAddr)

			dr := &DiscoveryResult{
				JobID:           jobID,
				IPAddress:       ipAddr,
				MACAddress:      ptrString(mac),
				Manufacturer:    ptrString(manufacturer),
				Hostname:        ptrString(hostname),
				GuessedCategory: ptrString(category),
				OpenPorts:       openPorts,
				ResponseTimeMs:  respMs,
				Status:          "pending",
			}

			if scanType == "full" || scanType == "ping_snmp" {
				hasHTTP := portHas(openPorts, 80) || portHas(openPorts, 443) || portHas(openPorts, 8080) || portHas(openPorts, 8443)
				hasSSH := portHas(openPorts, 22)
				hasHTTPS := portHas(openPorts, 443) || portHas(openPorts, 8443)
				hasSNMP := portHas(openPorts, 161)

				if hasHTTP || hasHTTPS {
					dr.HTTPTitle = probeHTTPTitle(ctx, ipAddr, hasHTTPS)
				}
				if hasSSH {
					dr.SSHBanner = probeSSHBanner(ctx, ipAddr)
				}
				if hasHTTPS {
					dr.TLSCertCN = probeTLSCertCN(ctx, ipAddr)
				}
				if hasSNMP {
					name, desc, sysObjID := probeSNMP(ctx, ipAddr)
					dr.SNMPName = ptrString(name)
					dr.SNMPDescription = ptrString(desc)
					dr.SNMPSysObjectID = ptrString(sysObjID)
					dr.SNMPReachable = true
				}
			}

			enrichFromProbes(dr)
			results[idx] = ipResult{result: dr}
			devicesFound.Add(1)
		}(i, ip)
	}
	wg.Wait()
	var newCount, knownCount int64
	for _, r := range results {
		if r.result == nil {
			continue
		}
		if err := s.insertResult(ctx, r.result); err != nil {
			slog.Error("failed to insert discovery result", "ip", r.result.IPAddress, "error", err)
			continue
		}
		if r.result.IsKnown {
			knownCount++
		} else {
			newCount++
		}
	}
	found := devicesFound.Load()
	_, err = s.pool.Exec(ctx,
		`UPDATE discovery_jobs SET devices_found = $1, devices_new = $2, devices_known = $3,
		 status = 'completed', completed_at = now()
		 WHERE id = $4`,
		found, newCount, knownCount, jobID)
	if err != nil {
		s.updateJobStatus(ctx, jobID, "failed", err.Error())
		return err
	}
	return nil
}

func (s *Scanner) updateJobStatus(ctx context.Context, jobID int64, status, errMsg string) {
	if errMsg != "" {
		_, _ = s.pool.Exec(ctx,
			`UPDATE discovery_jobs SET status = $1, error_message = $2, completed_at = now() WHERE id = $3`,
			status, errMsg, jobID)
	} else {
		_, _ = s.pool.Exec(ctx,
			`UPDATE discovery_jobs SET status = $1 WHERE id = $2`,
			status, jobID)
	}
}

func (s *Scanner) getKnownIPs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT ip_address FROM devices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		known[ip] = true
	}
	return known, rows.Err()
}

func (s *Scanner) insertResult(ctx context.Context, dr *DiscoveryResult) error {
	if dr.IPAddress == "" {
		return fmt.Errorf("ip_address is required")
	}
	var knownDeviceID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM devices WHERE ip_address = $1 LIMIT 1`,
		dr.IPAddress).Scan(&knownDeviceID)
	if err == nil && knownDeviceID != nil {
		dr.IsKnown = true
	}
	portsJSON, err := json.Marshal(dr.OpenPorts)
	if err != nil {
		portsJSON = []byte("[]")
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO discovery_results
			(job_id, ip_address, mac_address, manufacturer, hostname,
			 device_description, guessed_category, guessed_os, open_ports,
			 snmp_reachable, response_time_ms, status,
			 http_title, ssh_banner, tls_cert_cn,
			 snmp_name, snmp_description, snmp_sys_object_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		dr.JobID, dr.IPAddress, dr.MACAddress, dr.Manufacturer, dr.Hostname,
		dr.DeviceDescription, dr.GuessedCategory, dr.GuessedOS, string(portsJSON),
		dr.SNMPReachable, dr.ResponseTimeMs, dr.Status,
		dr.HTTPTitle, dr.SSHBanner, dr.TLSCertCN,
		dr.SNMPName, dr.SNMPDescription, dr.SNMPSysObjectID)
	return err
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type DiscoveryResult struct {
	ID                int64   `json:"id"`
	JobID             int64   `json:"jobId"`
	IPAddress         string  `json:"ipAddress"`
	MACAddress        *string `json:"macAddress,omitempty"`
	Manufacturer      *string `json:"manufacturer,omitempty"`
	Hostname          *string `json:"hostname,omitempty"`
	DeviceDescription *string `json:"deviceDescription,omitempty"`
	GuessedCategory   *string `json:"guessedCategory,omitempty"`
	GuessedOS         *string `json:"guessedOS,omitempty"`
	OpenPorts         []int   `json:"openPorts"`
	SNMPReachable     bool    `json:"snmpReachable"`
	ResponseTimeMs    float64 `json:"responseTimeMs"`
	Status            string  `json:"status"`
	ApprovedDeviceID  *int64  `json:"approvedDeviceId,omitempty"`
	IsKnown           bool    `json:"isKnown"`
	LocationID        *int64  `json:"locationId,omitempty"`
	HTTPTitle         *string `json:"httpTitle,omitempty"`
	SSHBanner         *string `json:"sshBanner,omitempty"`
	TLSCertCN         *string `json:"tlsCertCn,omitempty"`
	SNMPName          *string `json:"snmpName,omitempty"`
	SNMPDescription   *string `json:"snmpDescription,omitempty"`
	SNMPSysObjectID   *string `json:"snmpSysObjectID,omitempty"`
}

type DiscoveryJob struct {
	ID              int64      `json:"id"`
	Subnet          string     `json:"subnet"`
	ScanType        string     `json:"scanType"`
	Status          string     `json:"status"`
	LocationID      *int64     `json:"locationId,omitempty"`
	InitiatedBy     *string    `json:"initiatedBy,omitempty"`
	TotalIPsScanned int        `json:"totalIpsScanned"`
	DevicesFound    int        `json:"devicesFound"`
	DevicesNew      int        `json:"devicesNew"`
	DevicesKnown    int        `json:"devicesKnown"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
}

type StartScanRequest struct {
	Subnet       string `json:"subnet"`
	ScanType     string `json:"scanType"`
	LocationID   *int64 `json:"locationId,omitempty"`
	InitiatedBy  string `json:"initiatedBy,omitempty"`
	ExcludeKnown *bool  `json:"excludeKnown,omitempty"`
}

func (r *StartScanRequest) Validate() error {
	if r.Subnet == "" {
		return fmt.Errorf("subnet is required")
	}
	_, _, err := net.ParseCIDR(r.Subnet)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	switch r.ScanType {
	case "", "ping_only", "ping_snmp", "full":
		if r.ScanType == "" {
			r.ScanType = "full"
		}
	default:
		return fmt.Errorf("scan_type must be one of: ping_only, ping_snmp, full")
	}
	return nil
}

func expandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		dst := make(net.IP, len(ip))
		copy(dst, ip)
		ips = append(ips, dst.String())
	}
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func pingHost(ctx context.Context, ip string) (bool, float64) {
	pingCtx, cancel := context.WithTimeout(ctx, (pingTimeoutSec+1)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(pingCtx, "ping", "-c", "1", "-W", strconv.Itoa(pingTimeoutSec), ip) //nolint:gosec // IP from subnet scan, validated by net.ParseIP
	start := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Seconds() * 1000
	if err != nil {
		return false, elapsed
	}
	out := string(output)
	if strings.Contains(out, "bytes from") || strings.Contains(out, "ttl=") {
		respMs := parsePingRTT(out)
		if respMs > 0 {
			return true, respMs
		}
		return true, elapsed
	}
	return false, elapsed
}

var pingRTTRegex = regexp.MustCompile(`time[=><](\d+\.?\d*)`)

func parsePingRTT(output string) float64 {
	matches := pingRTTRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return v
		}
	}
	return 0
}

func refreshARPTable() {
	cmd := exec.Command("arp", "-a")
	_ = cmd.Run()
	time.Sleep(arpRefreshDelay)
}

var arpEntryRegex = regexp.MustCompile(`\((\d+\.\d+\.\d+\.\d+)\)\s+at\s+([0-9a-fA-F:]{17})`)

func lookupARP(ip string) string {
	cmd := exec.Command("arp", "-a", ip) //nolint:gosec // IP from subnet scan, validated by net.ParseIP
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	matches := arpEntryRegex.FindAllStringSubmatch(string(output), -1)
	for _, m := range matches {
		if len(m) >= 3 && m[1] == ip {
			return strings.ToLower(m[2])
		}
	}
	return ""
}

func ouiLookup(mac string) string {
	if len(mac) < 8 {
		return ""
	}
	prefix := strings.ToUpper(mac[:8])
	if mfr, ok := ouiTable[prefix]; ok {
		return mfr
	}
	prefix = strings.ToUpper(mac[:5])
	for k, v := range ouiTable {
		if strings.HasPrefix(k, prefix) {
			return v
		}
	}
	return ""
}

func guessCategoryFromPorts(ports []int) string {
	portSet := make(map[int]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}
	if portSet[515] || portSet[9100] {
		return "printer"
	}
	if portSet[554] {
		return "camera"
	}
	if portSet[3389] {
		return "workstation"
	}
	if portSet[161] {
		if portSet[22] || portSet[80] || portSet[443] {
			return "managed_switch"
		}
		return "snmp_device"
	}
	if portSet[22] && (portSet[80] || portSet[443]) {
		return "server"
	}
	if portSet[22] {
		return "server"
	}
	if portSet[80] || portSet[443] {
		return "server"
	}
	if portSet[21] || portSet[23] || portSet[25] || portSet[53] {
		return "server"
	}
	return "unknown"
}

func probeTCPPorts(ctx context.Context, host string, ports []int) []int {
	type portResult struct {
		port int
		open bool
	}
	results := make([]portResult, len(ports))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i, port := range ports {
		wg.Add(1)
		go func(idx, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				return
			default:
			}
			addr := net.JoinHostPort(host, strconv.Itoa(p))
			conn, err := net.DialTimeout("tcp", addr, tcpTimeout)
			if err != nil {
				return
			}
			_ = conn.Close()
			results[idx] = portResult{port: p, open: true}
		}(i, port)
	}
	wg.Wait()
	var openPorts []int
	for _, r := range results {
		if r.open {
			openPorts = append(openPorts, r.port)
		}
	}
	return openPorts
}

func reverseLookup(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

func excludeIPs(all []string, known map[string]bool) []string {
	var filtered []string
	for _, ip := range all {
		if !known[ip] {
			filtered = append(filtered, ip)
		}
	}
	return filtered
}

func portHas(ports []int, target int) bool {
	for _, p := range ports {
		if p == target {
			return true
		}
	}
	return false
}

func probeHTTPTitle(ctx context.Context, ip string, https bool) *string {
	scheme := "http"
	port := 80
	if https {
		scheme = "https"
		port = 443
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Intentional: scanning unknown network devices
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	url := fmt.Sprintf("%s://%s:%d/", scheme, ip, port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Rayavriti-Discovery/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if https && !https {
			return nil
		}
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Read up to 8KB to find the <title> tag
	limited := io.LimitReader(resp.Body, 8192)
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 4096), 8192)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "<title") {
			title := extractTitle(line)
			if title != "" {
				return &title
			}
		}
		if strings.Contains(strings.ToLower(line), "</head>") {
			break
		}
	}
	return nil
}

var titleRegex = regexp.MustCompile(`(?i)<title[^>]*>\s*([^<]+?)\s*</title>`)

func extractTitle(html string) string {
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) >= 2 {
		title := strings.TrimSpace(matches[1])
		if len(title) > 120 {
			title = title[:120]
		}
		return title
	}
	return ""
}

func probeSSHBanner(ctx context.Context, ip string) *string {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "22"))
	if err != nil {
		return nil
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return nil
	}
	banner := strings.TrimSpace(string(buf[:n]))
	if len(banner) > 120 {
		banner = banner[:120]
	}
	return &banner
}

func probeTLSCertCN(ctx context.Context, ip string) *string {
	for _, port := range []int{443, 8443} {
		dialer := &net.Dialer{Timeout: 3 * time.Second}
		addr := net.JoinHostPort(ip, strconv.Itoa(port))
		rawConn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // Intentional: scanning unknown network devices
		})
		if err != nil {
			continue
		}
		defer func() { _ = rawConn.Close() }()

		cert := rawConn.ConnectionState().PeerCertificates
		if len(cert) == 0 {
			continue
		}
		cn := cert[0].Subject.CommonName
		if cn != "" {
			if len(cn) > 120 {
				cn = cn[:120]
			}
			return &cn
		}
	}
	return nil
}

func probeSNMP(ctx context.Context, ip string) (name, description, sysObjectID string) {
	// SNMPv2 GET for sysName.0, sysDescr.0, sysObjectID.0
	// Using a minimal SNMPv2c GET request with community "public"
	oids := []string{
		"1.3.6.1.2.1.1.5.0", // sysName.0
		"1.3.6.1.2.1.1.1.0", // sysDescr.0
		"1.3.6.1.2.1.1.2.0", // sysObjectID.0
	}

	for i, oid := range oids {
		val := snmpGet(ctx, ip, oid)
		if val != "" {
			switch i {
			case 0:
				name = val
			case 1:
				description = val
			case 2:
				sysObjectID = val
			}
		}
	}
	return
}

func snmpGet(ctx context.Context, ip, oid string) string {
	// Build a minimal SNMPv2c GET request
	pkt := buildSNMPGetRequest(oid)
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(ip, "161"))
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = conn.Write(pkt)
	if err != nil {
		return ""
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	return parseSNMPResponse(buf[:n])
}

func buildSNMPGetRequest(oid string) []byte {
	oidBytes := encodeOID(oid)
	pduValue := append([]byte{0x30}, encodeLength(len(oidBytes)+2)...)
	pduValue = append(pduValue, 0x06, byte(len(oidBytes))) //nolint:gosec // OID lengths and values are validated and bounded
	pduValue = append(pduValue, oidBytes...)
	pduValue = append(pduValue, 0x05, 0x00) // NULL

	getReq := append([]byte{0xa0}, encodeLength(len(pduValue)+10)...)
	getReq = append(getReq, 0x02, 0x01, 0x00) // request-id = 0
	getReq = append(getReq, 0x02, 0x01, 0x00) // error-status = 0
	getReq = append(getReq, 0x02, 0x01, 0x00) // error-index = 0
	getReq = append(getReq, pduValue...)

	Community := []byte("public")
	version := []byte{0x02, 0x01, 0x01} // SNMPv2c

	snmpBody := append(version, []byte{0x04, byte(len(Community))}...) //nolint:gosec // OID lengths and values are validated and bounded
	snmpBody = append(snmpBody, Community...)
	snmpBody = append(snmpBody, getReq...)

	pkt := append([]byte{0x30}, encodeLength(len(snmpBody))...)
	pkt = append(pkt, snmpBody...)
	return pkt
}

func encodeOID(oid string) []byte {
	parts := strings.Split(oid, ".")
	if len(parts) < 2 {
		return nil
	}
	var encoded []byte
	first := 40*atoi(parts[0]) + atoi(parts[1])
	encoded = append(encoded, byte(first)) //nolint:gosec // OID lengths and values are validated and bounded
	for _, p := range parts[2:] {
		val := atoi(p)
		encoded = append(encoded, encodeVarint(val)...)
	}
	return encoded
}

func encodeVarint(val int) []byte {
	if val < 128 {
		return []byte{byte(val)} //nolint:gosec // OID lengths and values are validated and bounded
	}
	var bytes []byte
	for val > 0 {
		b := byte(val & 0x7f)
		val >>= 7
		if val > 0 {
			b |= 0x80
		}
		bytes = append([]byte{b}, bytes...)
	}
	return bytes
}

func encodeLength(length int) []byte {
	if length < 128 {
		return []byte{byte(length)} //nolint:gosec // OID lengths and values are validated and bounded
	}
	var bytes []byte
	for length > 0 {
		b := byte(length & 0xff)
		length >>= 8
		bytes = append([]byte{b}, bytes...)
	}
	result := make([]byte, 1+len(bytes)+1)
	result[0] = byte(0x80 | len(bytes)) //nolint:gosec // Length is bounded by SNMP protocol
	copy(result[1:], bytes)
	return result
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func parseSNMPResponse(data []byte) string {
	// Find the Value octet in the response — look for the OCTET STRING tag 0x04 after the PDU
	// Simple parser: find the last 0x04 tag that represents the variable binding value
	for i := len(data) - 1; i >= 4; i-- {
		if data[i-1] == 0x04 { // OCTET STRING
			length := int(data[i])
			if i+1+length <= len(data) && length > 0 && length < 256 {
				val := string(data[i+1 : i+1+length])
				// Filter out binary garbage — only printable ASCII
				if isPrintable(val) {
					return strings.TrimSpace(val)
				}
			}
		}
	}
	return ""
}

func isPrintable(s string) bool {
	if len(s) == 0 {
		return false
	}
	printable := 0
	for _, r := range s {
		if r >= 32 && r < 127 {
			printable++
		}
	}
	return float64(printable)/float64(len(s)) > 0.6
}

func enrichFromProbes(dr *DiscoveryResult) {
	// Build a device description from the best available identification
	parts := []string{}

	if dr.SNMPDescription != nil && *dr.SNMPDescription != "" {
		desc := *dr.SNMPDescription
		// Truncate long SNMP descriptions
		if len(desc) > 200 {
			desc = desc[:200]
		}
		parts = append(parts, desc)
	}

	if dr.SSHBanner != nil && *dr.SSHBanner != "" {
		parts = append(parts, *dr.SSHBanner)
	}

	if dr.TLSCertCN != nil && *dr.TLSCertCN != "" {
		parts = append(parts, "cert:"+*dr.TLSCertCN)
	}

	if len(parts) > 0 {
		desc := strings.Join(parts, " | ")
		dr.DeviceDescription = &desc
	}

	// Enrich hostname from SNMP if not set via DNS
	if (dr.Hostname == nil || *dr.Hostname == "") && dr.SNMPName != nil && *dr.SNMPName != "" {
		dr.Hostname = dr.SNMPName
	}

	// Enrich category from probe data
	category := guessCategoryFromProbes(dr)
	if category != "" {
		dr.GuessedCategory = ptrString(category)
	}

	// Enrich OS from SSH banner
	if dr.GuessedOS == nil || *dr.GuessedOS == "" {
		if dr.SSHBanner != nil && *dr.SSHBanner != "" {
			os := guessOSFromSSHBanner(*dr.SSHBanner)
			if os != "" {
				dr.GuessedOS = ptrString(os)
			}
		}
	}
}

func guessCategoryFromProbes(dr *DiscoveryResult) string {
	category := ""
	if dr.GuessedCategory != nil {
		category = *dr.GuessedCategory
	}

	// SNMP-based: check sysObjectID for known vendor OIDs
	if dr.SNMPSysObjectID != nil {
		oid := *dr.SNMPSysObjectID
		if strings.HasPrefix(oid, "1.3.6.1.4.1.9") {
			return "cisco_device"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.11") {
			return "printer"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.25461") {
			return "firewall"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.2636") {
			return "managed_switch"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.14823") {
			return "access_point"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.12356") {
			return "firewall"
		}
		if strings.HasPrefix(oid, "1.3.6.1.4.1.171") {
			return "firewall"
		}
	}

	// HTTP title hints
	if dr.HTTPTitle != nil {
		title := strings.ToLower(*dr.HTTPTitle)
		if strings.Contains(title, "router") || strings.Contains(title, "gateway") {
			return "router"
		}
		if strings.Contains(title, "switch") {
			return "managed_switch"
		}
		if strings.Contains(title, "access point") || strings.Contains(title, "wifi") || strings.Contains(title, "wireless") {
			return "access_point"
		}
		if strings.Contains(title, "firewall") || strings.Contains(title, "fortigate") || strings.Contains(title, "sophos") {
			return "firewall"
		}
		if strings.Contains(title, "printer") || strings.Contains(title, "laserjet") || strings.Contains(title, "officejet") {
			return "printer"
		}
		if strings.Contains(title, "camera") || strings.Contains(title, "nvr") || strings.Contains(title, "dvr") {
			return "camera"
		}
		if strings.Contains(title, "nas") || strings.Contains(title, "synology") || strings.Contains(title, "qnap") {
			return "nas"
		}
		if strings.Contains(title, "unifi") || strings.Contains(title, "ubiquiti") {
			return "access_point"
		}
		if strings.Contains(title, "pfsense") || strings.Contains(title, "opnsense") {
			return "firewall"
		}
	}

	// TLS cert CN hints
	if dr.TLSCertCN != nil {
		cn := strings.ToLower(*dr.TLSCertCN)
		if strings.Contains(cn, "fortigate") || strings.Contains(cn, "sophos") {
			return "firewall"
		}
	}

	return category
}

func guessOSFromSSHBanner(banner string) string {
	lower := strings.ToLower(banner)
	if strings.Contains(lower, "openssh") {
		if strings.Contains(lower, "ubuntu") {
			return "Linux (Ubuntu)"
		}
		if strings.Contains(lower, "debian") {
			return "Linux (Debian)"
		}
		if strings.Contains(lower, "centos") || strings.Contains(lower, "red hat") || strings.Contains(lower, "redhat") {
			return "Linux (RHEL/CentOS)"
		}
		if strings.Contains(lower, "freebsd") {
			return "FreeBSD"
		}
		if strings.Contains(lower, "openbsd") {
			return "OpenBSD"
		}
		return "Linux/Unix"
	}
	if strings.Contains(lower, "cisco") {
		return "Cisco IOS"
	}
	if strings.Contains(lower, "microsoft") || strings.Contains(lower, "windows") {
		return "Windows"
	}
	if strings.Contains(lower, "dropbear") {
		return "Embedded/Linux"
	}
	if strings.Contains(lower, "libssh") {
		return "Embedded/SSH"
	}
	if strings.Contains(lower, "tp-link") || strings.Contains(lower, "tplink") {
		return "Embedded (TP-Link)"
	}
	if strings.Contains(lower, "mikrotik") {
		return "MikroTik RouterOS"
	}
	return ""
}
