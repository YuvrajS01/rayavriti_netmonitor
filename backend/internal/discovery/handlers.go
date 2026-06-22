package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type DiscoveryHandler struct {
	pool    *pgxpool.Pool
	scanner *Scanner
}

func NewDiscoveryHandler(db database.Database) *DiscoveryHandler {
	pp, ok := db.(database.PoolProvider)
	if !ok || pp.Pool() == nil {
		slog.Warn("DiscoveryHandler: database does not provide a pool, discovery features will be unavailable")
		return &DiscoveryHandler{}
	}
	pool := pp.Pool()
	return &DiscoveryHandler{
		pool:    pool,
		scanner: NewScanner(pool),
	}
}

func (h *DiscoveryHandler) StartScan(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	var req StartScanRequest
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	excludeKnown := true
	if req.ExcludeKnown != nil {
		excludeKnown = *req.ExcludeKnown
	}
	var jobID int64
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO discovery_jobs (subnet, scan_type, status, location_id, initiated_by)
		VALUES ($1, $2, 'pending', $3, $4)
		RETURNING id`,
		req.Subnet, req.ScanType, req.LocationID, req.InitiatedBy,
	).Scan(&jobID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create scan job: "+err.Error())
		return
	}
	go func() {
		ctx := context.Background()
		err := h.scanner.Scan(ctx, jobID, req.Subnet, req.ScanType, req.LocationID, excludeKnown)
		if err != nil && ctx.Err() == nil {
			slog.Error("discovery scan failed", "jobId", jobID, "error", err)
		}
	}()
	httputil.SendCreated(w, map[string]any{"jobId": jobID, "subnet": req.Subnet, "scanType": req.ScanType})
}

func (h *DiscoveryHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, subnet, scan_type, status, location_id, initiated_by,
		        total_ips_scanned, devices_found, devices_new, devices_known,
		        started_at, completed_at, error_message
		 FROM discovery_jobs ORDER BY started_at DESC LIMIT 200`)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var jobs []DiscoveryJob
	for rows.Next() {
		var j DiscoveryJob
		if err := rows.Scan(
			&j.ID, &j.Subnet, &j.ScanType, &j.Status, &j.LocationID, &j.InitiatedBy,
			&j.TotalIPsScanned, &j.DevicesFound, &j.DevicesNew, &j.DevicesKnown,
			&j.StartedAt, &j.CompletedAt, &j.ErrorMessage,
		); err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, jobs)
}

func (h *DiscoveryHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	var j DiscoveryJob
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, subnet, scan_type, status, location_id, initiated_by,
		        total_ips_scanned, devices_found, devices_new, devices_known,
		        started_at, completed_at, error_message
		 FROM discovery_jobs WHERE id = $1`, id,
	).Scan(
		&j.ID, &j.Subnet, &j.ScanType, &j.Status, &j.LocationID, &j.InitiatedBy,
		&j.TotalIPsScanned, &j.DevicesFound, &j.DevicesNew, &j.DevicesKnown,
		&j.StartedAt, &j.CompletedAt, &j.ErrorMessage,
	)
	if err == pgx.ErrNoRows {
		httputil.SendError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, j)
}

func (h *DiscoveryHandler) GetJobResults(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	statusFilter := r.URL.Query().Get("status")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, job_id, ip_address, mac_address, manufacturer, hostname,
		        device_description, guessed_category, guessed_os, open_ports,
		        snmp_reachable, response_time_ms, status, approved_device_id,
		        http_title, ssh_banner, tls_cert_cn,
		        snmp_name, snmp_description, snmp_sys_object_id
		 FROM discovery_results WHERE job_id = $1
		 ORDER BY ip_address`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var results []DiscoveryResult
	for rows.Next() {
		var dr DiscoveryResult
		var openPortsJSON []byte
		var mac, mfg, host, desc, cat, os *string
		var httpTitle, sshBanner, tlsCertCN *string
		var snmpName, snmpDesc, snmpObjID *string
		if err := rows.Scan(
			&dr.ID, &dr.JobID, &dr.IPAddress, &mac, &mfg,
			&host, &desc, &cat, &os,
			&openPortsJSON, &dr.SNMPReachable, &dr.ResponseTimeMs, &dr.Status,
			&dr.ApprovedDeviceID,
			&httpTitle, &sshBanner, &tlsCertCN,
			&snmpName, &snmpDesc, &snmpObjID,
		); err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		dr.MACAddress = mac
		dr.Manufacturer = mfg
		dr.Hostname = host
		dr.DeviceDescription = desc
		dr.GuessedCategory = cat
		dr.GuessedOS = os
		dr.HTTPTitle = httpTitle
		dr.SSHBanner = sshBanner
		dr.TLSCertCN = tlsCertCN
		dr.SNMPName = snmpName
		dr.SNMPDescription = snmpDesc
		dr.SNMPSysObjectID = snmpObjID
		if openPortsJSON != nil {
			_ = json.Unmarshal(openPortsJSON, &dr.OpenPorts)
		}
		if statusFilter != "" && dr.Status != statusFilter {
			continue
		}
		results = append(results, dr)
	}
	if err := rows.Err(); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, results)
}

func (h *DiscoveryHandler) ApproveResult(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid result id")
		return
	}
	var req struct {
		Name       string  `json:"name"`
		LocationID *int64  `json:"locationId,omitempty"`
		Tags       []string `json:"tags,omitempty"`
	}
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var dr DiscoveryResult
	var openPortsJSON []byte
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, job_id, ip_address, mac_address, manufacturer, hostname,
		        device_description, guessed_category, guessed_os, open_ports,
		        snmp_reachable, response_time_ms, status, location_id
		 FROM discovery_results WHERE id = $1`, id,
	).Scan(
		&dr.ID, &dr.JobID, &dr.IPAddress, &dr.MACAddress, &dr.Manufacturer,
		&dr.Hostname, &dr.DeviceDescription, &dr.GuessedCategory, &dr.GuessedOS,
		&openPortsJSON, &dr.SNMPReachable, &dr.ResponseTimeMs, &dr.Status,
		&dr.LocationID,
	)
	if err == pgx.ErrNoRows {
		httputil.SendError(w, http.StatusNotFound, "result not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dr.Status == "approved" {
		httputil.SendError(w, http.StatusConflict, "result already approved")
		return
	}
	deviceName := req.Name
	if deviceName == "" {
		if dr.Hostname != nil && *dr.Hostname != "" {
			deviceName = *dr.Hostname
		} else {
			deviceName = dr.IPAddress
		}
	}
	locationID := req.LocationID
	if locationID == nil {
		locationID = dr.LocationID
	}
	tagsJSON, _ := json.Marshal(req.Tags)
	var protocol string
	var port int
	for _, p := range dr.OpenPorts {
		switch p {
		case 161:
			protocol = "snmp"
			port = 161
		case 80:
			if protocol == "" {
				protocol = "http"
				port = 80
			}
		case 443:
			if protocol == "" {
				protocol = "https"
				port = 443
			}
		case 22:
			if protocol == "" {
				protocol = "ssh"
				port = 22
			}
		case 3389:
			if protocol == "" {
				protocol = "rdp"
				port = 3389
			}
		}
	}
	if protocol == "" {
		protocol = "icmp"
		port = 0
	}
	var deviceID int64
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO devices (name, ip_address, protocol, port, enabled, status, tags,
		                     mac_address, manufacturer, device_category, location_id,
		                     snmp_community, snmp_version, snmp_port)
		VALUES ($1, $2, $3, $4, true, 'unknown', $5,
		        $6, $7, $8, $9,
		        'public', '2c', 161)
		RETURNING id`,
		deviceName, dr.IPAddress, protocol, port, string(tagsJSON),
		dr.MACAddress, dr.Manufacturer, dr.GuessedCategory, locationID,
	).Scan(&deviceID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create device: "+err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE discovery_results SET status = 'approved', approved_device_id = $1 WHERE id = $2`,
		deviceID, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to update result: "+err.Error())
		return
	}
	httputil.SendCreated(w, map[string]any{"deviceId": deviceID, "resultId": id})
}

func (h *DiscoveryHandler) RejectResult(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid result id")
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE discovery_results SET status = 'rejected' WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.SendError(w, http.StatusNotFound, "result not found or not pending")
		return
	}
	httputil.SendOK(w, map[string]any{"resultId": id, "status": "rejected"})
}

func (h *DiscoveryHandler) BulkApprove(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "discovery service unavailable")
		return
	}
	var req struct {
		ResultIDs  []int64 `json:"resultIds"`
		LocationID *int64  `json:"locationId,omitempty"`
	}
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.ResultIDs) == 0 {
		httputil.SendError(w, http.StatusBadRequest, "resultIds is required")
		return
	}
	idsStr := make([]string, len(req.ResultIDs))
	args := make([]any, len(req.ResultIDs))
	for i, id := range req.ResultIDs {
		idsStr[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	placeholders := strings.Join(idsStr, ",")
	query := `SELECT id, job_id, ip_address, mac_address, manufacturer, hostname,
	                 device_description, guessed_category, guessed_os, open_ports,
	                 snmp_reachable, response_time_ms, status, location_id
	          FROM discovery_results WHERE id IN (` + placeholders + `) AND status = 'pending'`
	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var toApprove []DiscoveryResult
	for rows.Next() {
		var dr DiscoveryResult
		var openPortsJSON []byte
		if err := rows.Scan(
			&dr.ID, &dr.JobID, &dr.IPAddress, &dr.MACAddress, &dr.Manufacturer,
			&dr.Hostname, &dr.DeviceDescription, &dr.GuessedCategory, &dr.GuessedOS,
			&openPortsJSON, &dr.SNMPReachable, &dr.ResponseTimeMs, &dr.Status,
			&dr.LocationID,
		); err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if openPortsJSON != nil {
			_ = json.Unmarshal(openPortsJSON, &dr.OpenPorts)
		}
		toApprove = append(toApprove, dr)
	}
	if err := rows.Err(); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type approvedItem struct {
		ResultID int64 `json:"resultId"`
		DeviceID int64 `json:"deviceId"`
	}
	var approved []approvedItem
	var failed []map[string]any
	for _, dr := range toApprove {
		deviceName := dr.IPAddress
		if dr.Hostname != nil && *dr.Hostname != "" {
			deviceName = *dr.Hostname
		}
		locationID := req.LocationID
		if locationID == nil {
			locationID = dr.LocationID
		}
		var protocol string
		var port int
		for _, p := range dr.OpenPorts {
			switch p {
			case 161:
				protocol = "snmp"
				port = 161
			case 80:
				if protocol == "" {
					protocol = "http"
					port = 80
				}
			case 443:
				if protocol == "" {
					protocol = "https"
					port = 443
				}
			case 22:
				if protocol == "" {
					protocol = "ssh"
					port = 22
				}
			case 3389:
				if protocol == "" {
					protocol = "rdp"
					port = 3389
				}
			}
		}
		if protocol == "" {
			protocol = "icmp"
		}
		var deviceID int64
		err := h.pool.QueryRow(r.Context(), `
			INSERT INTO devices (name, ip_address, protocol, port, enabled, status,
			                     mac_address, manufacturer, device_category, location_id,
			                     snmp_community, snmp_version, snmp_port)
			VALUES ($1, $2, $3, $4, true, 'unknown',
			        $5, $6, $7, $8,
			        'public', '2c', 161)
			RETURNING id`,
			deviceName, dr.IPAddress, protocol, port,
			dr.MACAddress, dr.Manufacturer, dr.GuessedCategory, locationID,
		).Scan(&deviceID)
		if err != nil {
			failed = append(failed, map[string]any{"resultId": dr.ID, "error": err.Error()})
			slog.Error("bulk approve: failed to create device", "resultId", dr.ID, "error", err)
			continue
		}
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE discovery_results SET status = 'approved', approved_device_id = $1 WHERE id = $2`,
			deviceID, dr.ID)
		approved = append(approved, approvedItem{ResultID: dr.ID, DeviceID: deviceID})
	}
	httputil.SendOK(w, map[string]any{
		"approved": approved,
		"failed":   failed,
		"total":    len(req.ResultIDs),
	})
}
