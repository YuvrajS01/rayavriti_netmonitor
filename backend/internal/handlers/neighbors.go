package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

// neighborSelect resolves each LLDP neighbour to a monitored device: by
// management address first, then by system name (also its first DNS label,
// since agents often report an FQDN). Resolution happens at read time so it
// follows devices being added, renamed or readdressed.
const neighborSelect = `
	SELECT n.device_id, n.local_port_num, n.local_port, n.remote_chassis_id, n.remote_sys_name,
	       n.remote_port_id, n.remote_port_desc, n.remote_mgmt_ip, rd.id, COALESCE(rd.name, ''),
	       n.first_seen, n.last_seen
	FROM device_neighbors n
	LEFT JOIN LATERAL (
		SELECT d.id, d.name FROM devices d
		WHERE d.id <> n.device_id AND (
		      (n.remote_mgmt_ip <> '' AND d.ip_address = n.remote_mgmt_ip)
		   OR (n.remote_sys_name <> '' AND (lower(d.name) = lower(n.remote_sys_name)
		                                OR lower(d.name) = split_part(lower(n.remote_sys_name), '.', 1))))
		ORDER BY (d.ip_address = n.remote_mgmt_ip) DESC, d.id
		LIMIT 1
	) rd ON TRUE`

func (h *InterfaceHandler) queryNeighbors(ctx context.Context, where string, args ...any) ([]models.LLDPNeighbor, error) {
	rows, err := h.pool.Query(ctx, neighborSelect+where+` ORDER BY n.device_id, n.local_port_num, n.remote_sys_name`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.LLDPNeighbor, error) {
		var n models.LLDPNeighbor
		err := row.Scan(&n.DeviceID, &n.LocalPortNum, &n.LocalPort, &n.RemoteChassisID, &n.RemoteSysName,
			&n.RemotePortID, &n.RemotePortDesc, &n.RemoteMgmtIP, &n.RemoteDeviceID, &n.RemoteDevice,
			&n.FirstSeen, &n.LastSeen)
		return n, err
	})
}

// Neighbors lists the LLDP neighbours a device reported. The route sits
// behind DeviceScopeGuard; for scoped callers the remote is not resolved to
// a device, which could be outside their scope.
func (h *InterfaceHandler) Neighbors(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	neighbors, err := h.queryNeighbors(r.Context(), ` WHERE n.device_id = $1`, id)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if sc := rbac.GetScopeContext(r); sc != nil && sc.IsScoped {
		for i := range neighbors {
			neighbors[i].RemoteDeviceID = nil
			neighbors[i].RemoteDevice = ""
		}
	}
	if neighbors == nil {
		neighbors = []models.LLDPNeighbor{}
	}
	httputil.SendOK(w, neighbors)
}

// topologyLink is a physical link discovered over LLDP. ToDeviceID is nil
// when the far end is not a monitored device; ToName then carries what the
// neighbour reported about itself.
type topologyLink struct {
	FromDeviceID int64  `json:"fromDeviceId"`
	FromPort     string `json:"fromPort"`
	ToDeviceID   *int64 `json:"toDeviceId"`
	ToName       string `json:"toName"`
	ToPort       string `json:"toPort"`
	ToMgmtIP     string `json:"toMgmtIp,omitempty"`
}

// Links returns the LLDP topology as links between devices. A link seen
// from both ends (each side reports the other) is returned once.
// Neighbours arrive ordered by device_id, so the lower id's view wins.
func (h *InterfaceHandler) Links(w http.ResponseWriter, r *http.Request) {
	neighbors, err := h.queryNeighbors(r.Context(), "")
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, neighborLinks(neighbors))
}

func neighborLinks(neighbors []models.LLDPNeighbor) []topologyLink {
	links := make([]topologyLink, 0, len(neighbors))
	reported := map[string]bool{}
	for _, n := range neighbors {
		toPort := n.RemotePortDesc
		if toPort == "" {
			toPort = n.RemotePortID
		}
		if n.RemoteDeviceID != nil {
			// Keep every link the first-seen side reports (parallel links
			// stay separate) and drop the far side's reports of them.
			if reported[fmt.Sprintf("%d|%d", *n.RemoteDeviceID, n.DeviceID)] {
				continue
			}
			reported[fmt.Sprintf("%d|%d", n.DeviceID, *n.RemoteDeviceID)] = true
		}
		name := n.RemoteDevice
		if name == "" {
			name = n.RemoteSysName
		}
		if name == "" {
			name = n.RemoteChassisID
		}
		links = append(links, topologyLink{
			FromDeviceID: n.DeviceID, FromPort: n.LocalPort,
			ToDeviceID: n.RemoteDeviceID, ToName: name, ToPort: toPort, ToMgmtIP: n.RemoteMgmtIP,
		})
	}
	return links
}
