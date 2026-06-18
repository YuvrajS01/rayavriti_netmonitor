# Rayavriti NetMonitor — Phase 2: Campus-Grade Features

> **Prerequisite**: All features in this document are to be implemented **after** the Go backend migration (Phase 1) described in [implementation_plan.md](file:///home/yuvraj/.gemini/antigravity/brain/5f71f6dd-3695-4c2e-8e0d-e5bed490147f/implementation_plan.md) is complete and verified.

This plan transforms Rayavriti NetMonitor from a generic network monitor into a **purpose-built campus network monitoring platform** — designed for colleges, universities, and institutions where hundreds of static-IP LAN ports, switches, routers, and services must be tracked, with alerts routed to the right person and reports delivered to management.

---

## User Review Required

> [!IMPORTANT]
> **Database schema extension**: Phase 2 adds **30+ new tables** and extends the `devices` and `users` tables with new columns. All changes are PostgreSQL migrations layered on top of the Phase 1 PostgreSQL + TimescaleDB schema. Existing data is preserved, but migrations must be versioned, transactional where PostgreSQL allows, and verified against a backup before production rollout.

> [!WARNING]
> **Frontend changes required**: Unlike Phase 1 (backend-only migration), Phase 2 requires **8+ new React pages** in the client — Campus Map, Incident Manager, Contact Directory, Status Page admin, Maintenance Calendar, RBAC user management, Report Builder, and Discovery Dashboard. These are net-new UI features.

> [!IMPORTANT]
> **Telegram Bot networking**: Telegram webhooks require a publicly reachable HTTPS endpoint. In a college behind NAT, long polling mode is recommended and does not require a public IP. The bot token must be created via @BotFather and kept secret.

> [!IMPORTANT]
> **RBAC is a breaking change for the frontend and realtime events**: Every API call and WebSocket event must be filtered by user scope. The React client needs to adapt — hiding UI elements the user can't access, handling 403 responses gracefully, and subscribing only to permitted realtime streams. This touches every existing page.

## Decisions & Open Questions

> [!IMPORTANT]
> **1. Telegram vs other bots**: Should we implement Telegram Bot (recommended for India), or would you prefer Discord Bot (more common in some colleges)? Or both?

> [!IMPORTANT]
> **2. PDF generation approach**: For SLA reports, should we use:
> - **A) Pure Go PDF** (`gofpdf`) — No external deps, but limited styling
> - **B) HTML template → PDF** (via headless Chrome/`rod`) — Beautiful output, but heavier dependency
> - **Recommendation**: Option A for simplicity, upgrade to B later if needed

> [!IMPORTANT]
> **3. Public status page design**: Should the `/status` page be:
> - **A) Server-rendered HTML** — Works without the React client, lightweight
> - **B) Part of the React SPA** — Consistent design, but requires serving the full app bundle
> - **Recommendation**: Option A (standalone HTML) for fastest load and no-auth simplicity

> [!IMPORTANT]
> **4. RBAC enforcement scope decision**: RBAC must be enforced at both the API and WebSocket layers. A `dept_admin` only receives `metric:update`, `alert:triggered`, and `device:status` events for devices in their location scope. This is required to avoid bypassing REST authorization through realtime updates.

> [!IMPORTANT]
> **5. On-premise vs cloud notifications**: For SMS and WhatsApp, should we support:
> - **A) Cloud APIs only** (Twilio, MSG91, Gupshup) — requires internet
> - **B) Also support on-premise SMS gateways** (USB GSM modem via AT commands) — works during internet outage
> - **Recommendation**: Start with cloud APIs, add GSM modem support as optional later

---

## Database Migration Strategy

All Phase 2 schema changes are **additive and non-destructive**. They are applied through the Phase 1 PostgreSQL migration runner, not through `ensureColumn`. Each migration gets a monotonically increasing version in `schema_migrations`, uses PostgreSQL-native types (`BIGSERIAL`, `BIGINT`, `BOOLEAN`, `TIMESTAMPTZ`, `JSONB`, `INET`, `CIDR`), and includes a rollback note even when automatic rollback is not safe.

Migration order matters:
1. Create independent lookup tables (`roles`, `contacts`, `locations`, `sla_definitions`)
2. Add nullable foreign-key columns to existing tables
3. Create join tables and high-volume event tables
4. Backfill default roles, root location, and default policies
5. Add indexes and optional TimescaleDB hypertables/policies
6. Enable application code paths that depend on the new schema

### Existing Table Extensions

| Table | New Columns | Migration Method |
|---|---|---|
| `devices` | `location_id`, `parent_device_id`, `dependency_port`, `rack_position`, `asset_tag`, `mac_address`, `serial_number`, `manufacturer`, `model`, `device_category`, `notes` | Versioned `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, then FK/index creation |
| `users` | `role_id`, `display_name`, `email`, `phone`, `contact_id`, `enabled`, `last_login_at` | Versioned `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, then backfill existing users to `super_admin` |

### New Tables (30+)

| Category | Tables Created |
|---|---|
| Locations | `locations`, `subnets` |
| Dependencies | `suppressed_alerts` |
| Discovery | `discovery_jobs`, `discovery_results` |
| Status Page | `status_page_services`, `status_page_service_devices`, `status_page_incidents`, `status_page_incident_services`, `status_page_incident_updates` |
| Maintenance | `maintenance_windows` |
| Contacts | `contacts`, `device_contacts`, `escalation_policies`, `escalation_steps`, `oncall_schedules` |
| Incidents | `incidents`, `incident_devices`, `incident_timeline`, `sla_definitions` |
| RBAC | `roles`, `user_scopes` |
| Notifications | `notification_log` |
| Reports | `scheduled_reports`, `generated_reports` |
| ISP | `isp_links`, `isp_metrics` |

### TimescaleDB Tables

Create regular PostgreSQL tables first, backfill any migrated data, then convert high-volume event/metric tables to Timescale hypertables:

```sql
SELECT create_hypertable('suppressed_alerts', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('notification_log', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('incident_timeline', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('isp_metrics', 'created_at', if_not_exists => TRUE);
```

Hypertable unique indexes must include the time partition column. If a table needs a globally unique ID and a hypertable, keep the `id` as a non-unique surrogate or include `created_at` in the primary/unique key.

### Seed Data

On first boot after Phase 2 upgrade:
- **5 system roles** seeded: `super_admin`, `network_admin`, `dept_admin`, `viewer`, `public`
- **3 SLA definitions** seeded: Critical (15min response / 2hr resolve), Major (30min / 8hr), Minor (1hr / 24hr)
- **1 root location** created: "Main Campus" (type: `campus`)
- **1 default escalation policy** created: "Default IT Escalation"
- **Existing admin user** assigned `super_admin` role
- **7 default alert rules** from Phase 1 linked to the default escalation policy

---

## Frontend Changes Required (React Client)

Phase 2 requires the following new pages and components in `client/src/`:

| New Page/Component | Route | Description |
|---|---|---|
| Campus Map / Topology | `/campus` | Interactive building → floor → room tree view with device status drill-down |
| Location Manager | `/settings/locations` | CRUD for location hierarchy with drag-and-drop reordering |
| Incident Dashboard | `/incidents` | Active incidents list, timeline view, create/resolve workflow |
| Contact Directory | `/settings/contacts` | Contact CRUD, device assignment, escalation policy builder |
| Status Page Admin | `/settings/status-page` | Configure which services appear on the public status page |
| Maintenance Calendar | `/maintenance` | Calendar view of maintenance windows, create/edit/delete |
| User Management | `/settings/users` | User CRUD, role assignment, location scope configuration |
| Report Builder | `/reports/builder` | Create scheduled reports, configure recipients, generate on-demand |
| Discovery Dashboard | `/discovery` | Launch scans, review results, approve/reject discovered devices |
| Bulk Import | `/import` | Upload CSV, preview validation results, confirm import |
| ISP Dashboard | `/isp` | ISP link metrics, SLA compliance charts, comparison view |
| Public Status Page | `/status` (standalone) | Server-rendered HTML, no auth, auto-refresh |

> [!NOTE]
> Frontend implementation details are not fully specified in this plan. Each page should follow the existing design system and component patterns in the React client. Detailed UI wireframes can be created during implementation.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                       Campus Dashboard                               │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ ┌────────────────────┐  │
│  │ Campus   │ │ Status    │ │ Incident   │ │ Reports &          │  │
│  │ Topology │ │ Page      │ │ Manager    │ │ SLA Dashboard      │  │
│  │ Map      │ │ (Public)  │ │            │ │                    │  │
│  └──────────┘ └───────────┘ └────────────┘ └────────────────────┘  │
└────────────────────────────┬────────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────────┐
│                       Go Backend                                     │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────────────┐  │
│  │ Location  │ │Dependency │ │ Escalation│ │ Report            │  │
│  │ Hierarchy │ │ Tree &    │ │ Engine &  │ │ Generator         │  │
│  │ Manager   │ │ Suppressor│ │ Contacts  │ │ (PDF/CSV/Email)   │  │
│  └───────────┘ └───────────┘ └───────────┘ └───────────────────┘  │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────────────┐  │
│  │ Auto      │ │Maintenance│ │ Incident  │ │ RBAC              │  │
│  │ Discovery │ │ Windows   │ │ Manager   │ │ Middleware         │  │
│  └───────────┘ └───────────┘ └───────────┘ └───────────────────┘  │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐                       │
│  │ Telegram  │ │ WhatsApp  │ │ SMS       │                       │
│  │ Bot       │ │ Business  │ │ Gateway   │                       │
│  └───────────┘ └───────────┘ └───────────┘                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## New Go Package Structure

All Phase 2 code lives inside `backend/internal/` alongside the Phase 1 packages:

```
backend/internal/
├── campus/
│   ├── location.go          # Location hierarchy CRUD & tree operations
│   ├── topology.go          # Dependency tree builder & visualizer data
│   └── discovery.go         # Auto-discovery: subnet scan, OUI lookup, SNMP probe
├── importer/
│   ├── csv_importer.go      # CSV/Excel bulk device import with validation
│   └── templates.go         # Import template generation
├── statuspage/
│   ├── statuspage.go        # Public status page data builder
│   └── handlers.go          # Unauthenticated /status endpoints
├── maintenance/
│   └── maintenance.go       # Maintenance window scheduler & suppression
├── contacts/
│   ├── contacts.go          # Contact directory CRUD
│   └── escalation.go        # Escalation policy engine & on-call rotation
├── incidents/
│   ├── incidents.go         # Incident lifecycle manager
│   └── timeline.go          # Incident timeline builder
├── rbac/
│   ├── rbac.go              # Permission evaluation engine
│   ├── middleware.go         # HTTP middleware for role-based filtering
│   └── roles.go             # Role definitions & permission matrix
├── notifications/
│   ├── telegram.go          # Telegram Bot API integration
│   ├── whatsapp.go          # WhatsApp Business API integration
│   └── sms.go               # SMS gateway integration
├── reports/
│   ├── generator.go         # Report data aggregation
│   ├── pdf.go               # PDF report renderer
│   ├── scheduler.go         # Cron-based report email scheduler
│   └── templates/           # HTML templates for PDF generation
├── handlers/
│   ├── locations.go         # Location hierarchy API handlers
│   ├── imports.go           # Bulk import API handlers
│   ├── status_page.go       # Public status page handlers
│   ├── maintenance.go       # Maintenance window handlers
│   ├── contacts.go          # Contact & escalation handlers
│   ├── incidents.go         # Incident management handlers
│   ├── discovery.go         # Auto-discovery handlers
│   └── reports_v2.go        # Advanced reporting handlers
└── services/
    ├── isp_monitor.go       # Dedicated ISP link health monitor
    └── service_checks.go    # College service templates (ERP, LMS, DNS, etc.)
```

#### New Go Dependencies

| Purpose | Package |
|---|---|
| PDF Generation | `github.com/jung-kurt/gofpdf` or `github.com/nicois/pdf` |
| CSV Parsing | `encoding/csv` (stdlib) |
| Excel Parsing | `github.com/xuri/excelize/v2` |
| Telegram Bot | `github.com/go-telegram-bot-api/telegram-bot-api/v5` |
| MAC OUI Lookup | `github.com/klauspost/oui` or embedded OUI database |
| ARP Scanning | `github.com/mdlayher/arp` |
| Cron (Reports) | `github.com/robfig/cron/v3` (already in Phase 1) |
| HTML → PDF | `github.com/nicois/pdf` or headless chromium via `rod` |

---

## Feature 1: Location Hierarchy & Campus Topology

### Purpose
Organize hundreds of devices into a navigable **Building → Floor → Room → Port** tree. Replace the flat device list with a spatial, drill-down interface.

### Database Schema

```sql
-- ── Location tree (recursive hierarchy) ─────────────────────────
CREATE TABLE IF NOT EXISTS locations (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Admin Block", "Ground Floor", "Room 101"
    type TEXT NOT NULL,                    -- 'campus', 'building', 'floor', 'room', 'rack', 'zone'
    parent_id BIGINT,                     -- NULL for root (campus), otherwise parent location
    code TEXT UNIQUE,                      -- Short code: "AB-GF-101" (for labels & reports)
    description TEXT,
    address TEXT,                          -- Physical address or directions
    latitude REAL,                         -- GPS coordinates (for campus map)
    longitude REAL,
    floor_number INTEGER,                  -- For floor-type locations
    contact_person_id BIGINT,             -- Default contact for this location
    metadata JSONB,                    -- Extensible: {"capacity": 40, "has_ac": true}
    sort_order INTEGER DEFAULT 0,          -- Display ordering among siblings
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (parent_id) REFERENCES locations(id) ON DELETE SET NULL,
    FOREIGN KEY (contact_person_id) REFERENCES contacts(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_loc_parent ON locations(parent_id);
CREATE INDEX IF NOT EXISTS idx_loc_type ON locations(type);
CREATE INDEX IF NOT EXISTS idx_loc_code ON locations(code);

-- ── Device-to-location mapping ──────────────────────────────────
-- Extends the existing `devices` table with a location_id column
ALTER TABLE devices ADD COLUMN IF NOT EXISTS location_id BIGINT REFERENCES locations(id) ON DELETE SET NULL;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS rack_position TEXT;        -- "Rack A, U12"
ALTER TABLE devices ADD COLUMN IF NOT EXISTS asset_tag TEXT;             -- "ASSET-2024-0142"
ALTER TABLE devices ADD COLUMN IF NOT EXISTS mac_address TEXT;           -- "AA:BB:CC:DD:EE:FF"
ALTER TABLE devices ADD COLUMN IF NOT EXISTS serial_number TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS manufacturer TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS model TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS device_category TEXT;       -- 'router', 'switch', 'access_point', 'server',
                                                           -- 'workstation', 'printer', 'cctv', 'biometric',
                                                           -- 'ups', 'lan_port', 'firewall', 'other'
ALTER TABLE devices ADD COLUMN IF NOT EXISTS notes TEXT;
CREATE INDEX IF NOT EXISTS idx_devices_location ON devices(location_id);
CREATE INDEX IF NOT EXISTS idx_devices_category ON devices(device_category);

-- ── VLAN / Subnet registry ──────────────────────────────────────
CREATE TABLE IF NOT EXISTS subnets (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "CS Labs VLAN"
    vlan_id BIGINT,                       -- 20
    cidr CIDR NOT NULL,                    -- "10.2.0.0/16"
    gateway INET,                          -- "10.2.0.1"
    description TEXT,
    location_id BIGINT,                   -- Primary location this subnet serves
    dns_servers INET[],                     -- {"10.0.0.53","10.0.0.54"}
    dhcp_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_subnets_vlan ON subnets(vlan_id);
```

### API Endpoints

```
# Location Hierarchy
GET    /api/v1/locations                     — List all locations (flat or tree mode)
       ?format=tree                          — Returns nested tree structure
       ?format=flat                          — Returns flat list with parent_id
       ?type=building                        — Filter by type
       ?parent_id=5                          — Children of a specific location

GET    /api/v1/locations/:id                 — Single location with children & device counts
GET    /api/v1/locations/:id/tree            — Full subtree from this location downward
GET    /api/v1/locations/:id/devices         — All devices at this location (recursive)
GET    /api/v1/locations/:id/status          — Aggregated health status for the location
POST   /api/v1/locations                     — Create location
PUT    /api/v1/locations/:id                 — Update location
DELETE /api/v1/locations/:id                 — Delete location (reassign children)
POST   /api/v1/locations/:id/move            — Move location to new parent

# Subnets
GET    /api/v1/subnets                       — List all subnets/VLANs
POST   /api/v1/subnets                       — Register a subnet
PUT    /api/v1/subnets/:id                   — Update subnet
DELETE /api/v1/subnets/:id                   — Delete subnet
GET    /api/v1/subnets/:id/devices           — Devices within this subnet (by IP range)

# Campus topology data (for visual map)
GET    /api/v1/topology                      — Full campus topology (locations + devices + links)
GET    /api/v1/topology/map                  — Simplified map data (for rendering)
```

### Response Shape: Location Tree

```json
{
  "id": 1,
  "name": "Main Campus",
  "type": "campus",
  "code": "MC",
  "children": [
    {
      "id": 2,
      "name": "Admin Block",
      "type": "building",
      "code": "AB",
      "deviceCount": 45,
      "status": { "up": 43, "down": 1, "warning": 1, "maintenance": 0 },
      "children": [
        {
          "id": 5,
          "name": "Ground Floor",
          "type": "floor",
          "code": "AB-GF",
          "floorNumber": 0,
          "deviceCount": 18,
          "status": { "up": 17, "down": 1, "warning": 0, "maintenance": 0 },
          "children": [...]
        }
      ]
    }
  ]
}
```

---

## Feature 2: Dependency Tree & Intelligent Alert Suppression

### Purpose
Model the physical network topology so that when a core switch dies, the system fires **one root-cause alert** instead of hundreds of individual device alerts.

### Database Schema

```sql
-- Add parent dependency to devices table
ALTER TABLE devices ADD COLUMN IF NOT EXISTS parent_device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dependency_port TEXT;   -- Which port on parent this device connects to
CREATE INDEX IF NOT EXISTS idx_devices_parent ON devices(parent_device_id);

-- Suppressed alerts tracking
CREATE TABLE IF NOT EXISTS suppressed_alerts (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL,
    rule_id BIGINT,
    suppression_reason TEXT NOT NULL,      -- 'parent_down', 'maintenance_window', 'dependency_outage'
    root_cause_device_id BIGINT,          -- The device that actually caused the outage
    root_cause_alert_id BIGINT,           -- The root cause alert
    would_have_fired_at TIMESTAMPTZ DEFAULT now(),
    released_at TIMESTAMPTZ,                  -- When suppression was lifted
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (device_id) REFERENCES devices(id),
    FOREIGN KEY (root_cause_device_id) REFERENCES devices(id),
    FOREIGN KEY (root_cause_alert_id) REFERENCES alerts(id)
);
CREATE INDEX IF NOT EXISTS idx_suppressed_device ON suppressed_alerts(device_id);
CREATE INDEX IF NOT EXISTS idx_suppressed_root ON suppressed_alerts(root_cause_device_id);
```

### Suppression Logic

```go
// Before creating an alert for a device, check its dependency chain:
func (e *AlertEngine) shouldSuppressAlert(device *models.Device) (*SuppressionResult, bool) {
    // 1. Walk up the parent chain
    parent := e.db.GetDevice(device.ParentDeviceID)
    for parent != nil {
        parentStatus := e.getLatestStatus(parent.ID)
        if parentStatus == "down" {
            return &SuppressionResult{
                Reason:          "parent_down",
                RootCauseDevice: parent,
                Message:         fmt.Sprintf("Suppressed: parent device %s (%s) is down",
                                    parent.Name, parent.Host),
            }, true
        }
        parent = e.db.GetDevice(parent.ParentDeviceID)
    }

    // 2. Check maintenance windows
    if e.maintenance.IsInWindow(device.ID) {
        return &SuppressionResult{
            Reason:  "maintenance_window",
            Message: "Suppressed: device is in maintenance window",
        }, true
    }

    return nil, false
}
```

### API Endpoints

```
GET    /api/v1/devices/:id/dependencies      — Get dependency chain (ancestors + descendants)
PUT    /api/v1/devices/:id/parent             — Set parent device
GET    /api/v1/topology/dependency-tree        — Full dependency tree for all devices
GET    /api/v1/alerts/suppressed               — List suppressed alerts with reasons
GET    /api/v1/outages/root-cause              — Current root-cause outages with affected device count
```

### Root-Cause Alert Shape

```json
{
  "alert_id": 891,
  "severity": "critical",
  "message": "Distribution Switch A (10.1.0.1) is DOWN",
  "device": { "id": 10, "name": "Distribution Switch A", "host": "10.1.0.1" },
  "affected_devices": {
    "total": 82,
    "by_category": { "switch": 3, "access_point": 5, "workstation": 60, "printer": 8, "cctv": 6 },
    "by_location": [
      { "location": "CS Lab 1", "count": 40 },
      { "location": "CS Lab 2", "count": 40 },
      { "location": "CS Faculty Room", "count": 2 }
    ]
  },
  "suppressed_alert_count": 82,
  "started_at": "2026-06-05T14:22:00Z"
}
```

---

## Feature 3: Bulk Device Import & Auto-Discovery

### 3A: CSV/Excel Import

```
POST /api/v1/import/devices
Content-Type: multipart/form-data
```

#### CSV Template

```csv
name,host,protocol,port,device_category,location_code,parent_device_host,mac_address,asset_tag,contact_email,notes
"Lab1-PC01",10.2.1.1,ping,,workstation,CS-L1-01,,AA:BB:CC:DD:EE:01,ASSET-001,sharma@college.edu,"Window seat row 1"
"Lab1-PC02",10.2.1.2,ping,,workstation,CS-L1-01,,AA:BB:CC:DD:EE:02,ASSET-002,sharma@college.edu,""
"Lab1-Switch",10.2.1.254,snmp,161,switch,CS-L1,10.2.0.1,AA:BB:CC:DD:EE:FE,ASSET-SW-01,rajesh@college.edu,"48-port managed"
```

#### Import Workflow

```
1. Upload CSV/Excel → POST /api/v1/import/devices
2. Server validates:
   - Required fields (name, host)
   - IP format validation
   - Duplicate detection (same host already exists?)
   - Location code exists?
   - Parent device host resolvable?
3. Return validation report → 200 OK with dry-run results
   {
     "total_rows": 340,
     "valid": 335,
     "warnings": 3,     // e.g., "location_code 'XX-YY' not found, will skip location assignment"
     "errors": 2,        // e.g., "row 142: invalid IP '10.2.1.999'"
     "duplicates": 5,    // existing devices with same host
     "preview": [first 10 rows with resolved data]
   }
4. User confirms → POST /api/v1/import/devices/confirm?import_id=xxx
5. Devices created in transaction with sensors, location assignment, parent linking
```

#### API Endpoints

```
GET    /api/v1/import/template               — Download CSV template with headers + example rows
POST   /api/v1/import/devices                 — Upload + validate (dry run)
POST   /api/v1/import/devices/confirm         — Execute validated import
GET    /api/v1/import/history                  — Past import jobs with stats
```

### 3B: Auto-Discovery

Scan a subnet and automatically find all live devices.

```
POST /api/v1/discovery/scan
{
  "subnet": "10.2.0.0/24",
  "scan_type": "full",            // "ping_only", "ping_snmp", "full"
  "snmp_community": "public",
  "location_id": 15,              // Assign discovered devices to this location
  "exclude_known": true           // Skip IPs already in device list
}
```

#### Discovery Pipeline

```
1. ICMP sweep → find all responding IPs
2. ARP table lookup → get MAC addresses
3. MAC OUI lookup → identify manufacturer (Cisco, HP, Dell, etc.)
4. SNMP probe (if scan_type includes SNMP):
   - sysDescr → device description
   - sysName → hostname
   - sysObjectID → device type
5. TCP port probe (common ports) → guess device role:
   - Port 80/443 open → "web server" or "managed switch"
   - Port 22 open → "Linux server"
   - Port 3389 open → "Windows workstation"
   - Port 161 open → "SNMP managed device"
   - Port 515/9100 open → "printer"
   - Port 554 open → "CCTV camera (RTSP)"
6. Results stored in discovery_results (pending approval)
```

#### Database Schema

```sql
CREATE TABLE IF NOT EXISTS discovery_jobs (
    id BIGSERIAL PRIMARY KEY,
    subnet TEXT NOT NULL,
    scan_type TEXT NOT NULL,
    status TEXT DEFAULT 'running',         -- 'running', 'completed', 'failed', 'cancelled'
    location_id BIGINT,
    initiated_by TEXT,
    total_ips_scanned INTEGER DEFAULT 0,
    devices_found INTEGER DEFAULT 0,
    devices_new INTEGER DEFAULT 0,          -- Not already in device list
    devices_known INTEGER DEFAULT 0,        -- Already monitored
    started_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    FOREIGN KEY (location_id) REFERENCES locations(id)
);

CREATE TABLE IF NOT EXISTS discovery_results (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL,
    ip_address TEXT NOT NULL,
    mac_address TEXT,
    manufacturer TEXT,                      -- From OUI lookup
    hostname TEXT,                          -- From SNMP sysName or reverse DNS
    device_description TEXT,               -- From SNMP sysDescr
    guessed_category TEXT,                 -- 'router', 'switch', 'workstation', etc.
    guessed_os TEXT,                        -- "Cisco IOS 15.2", "Windows 10", "Linux"
    open_ports JSONB,                        -- JSON array: [22, 80, 161, 443]
    snmp_reachable BOOLEAN DEFAULT false,
    response_time_ms REAL,
    status TEXT DEFAULT 'pending',          -- 'pending', 'approved', 'rejected', 'ignored'
    approved_device_id BIGINT,             -- Links to devices table after approval
    discovered_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (job_id) REFERENCES discovery_jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (approved_device_id) REFERENCES devices(id)
);
CREATE INDEX IF NOT EXISTS idx_disc_results_job ON discovery_results(job_id);
CREATE INDEX IF NOT EXISTS idx_disc_results_status ON discovery_results(status);
```

#### API Endpoints

```
POST   /api/v1/discovery/scan                — Start a discovery scan
GET    /api/v1/discovery/jobs                 — List past discovery jobs
GET    /api/v1/discovery/jobs/:id             — Job details + results
GET    /api/v1/discovery/jobs/:id/results     — Discovered devices for a job
POST   /api/v1/discovery/results/:id/approve  — Approve and add to monitoring
POST   /api/v1/discovery/results/:id/reject   — Reject / ignore a discovered device
POST   /api/v1/discovery/results/bulk-approve — Approve multiple devices at once
```

---

## Feature 4: Public Status Page

### Purpose
A clean, unauthenticated page that anyone on the campus network can visit to check if services are running. Reduces *"Is the internet working?"* calls to IT by 50%+.

### Database Schema

```sql
-- Admin-configurable: which services appear on the status page
CREATE TABLE IF NOT EXISTS status_page_services (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Internet Gateway"
    description TEXT,                      -- "Main campus internet connection"
    group_name TEXT NOT NULL DEFAULT 'General',  -- "Core Infrastructure", "Academic Services", "Student Services"
    aggregation TEXT DEFAULT 'any_down',   -- 'any_down' = red if ANY device is down
                                           -- 'all_down' = red only if ALL are down
                                           -- 'majority' = red if >50% down
    display_order INTEGER DEFAULT 0,
    show_response_time BOOLEAN DEFAULT false,  -- Show avg response time publicly?
    show_uptime BOOLEAN DEFAULT true,         -- Show uptime percentage?
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS status_page_service_devices (
    service_id BIGINT NOT NULL,
    device_id BIGINT NOT NULL,
    PRIMARY KEY (service_id, device_id),
    FOREIGN KEY (service_id) REFERENCES status_page_services(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_status_service_devices_device ON status_page_service_devices(device_id);

-- Public incident announcements
CREATE TABLE IF NOT EXISTS status_page_incidents (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,                    -- "Email server maintenance"
    message TEXT NOT NULL,                  -- "Scheduled maintenance on email server"
    severity TEXT DEFAULT 'info',           -- 'info', 'warning', 'critical'
    status TEXT DEFAULT 'investigating',    -- 'investigating', 'identified', 'monitoring', 'resolved'
    started_at TIMESTAMPTZ DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    created_by INTEGER,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS status_page_incident_services (
    incident_id BIGINT NOT NULL,
    service_id BIGINT NOT NULL,
    PRIMARY KEY (incident_id, service_id),
    FOREIGN KEY (incident_id) REFERENCES status_page_incidents(id) ON DELETE CASCADE,
    FOREIGN KEY (service_id) REFERENCES status_page_services(id) ON DELETE CASCADE
);

-- Updates within a public incident
CREATE TABLE IF NOT EXISTS status_page_incident_updates (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    created_by INTEGER,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (incident_id) REFERENCES status_page_incidents(id) ON DELETE CASCADE
);
```

### API Endpoints

```
# Public (NO AUTH)
GET    /status                                — HTML status page (server-rendered)
GET    /api/v1/public/status                  — JSON status data (for custom displays)
GET    /api/v1/public/status/history          — 90-day uptime history per service
GET    /api/v1/public/incidents               — Current and recent incidents

# Admin (AUTH REQUIRED)
GET    /api/v1/status-page/services           — List configured services
POST   /api/v1/status-page/services           — Add service to status page
PUT    /api/v1/status-page/services/:id       — Update service config
DELETE /api/v1/status-page/services/:id       — Remove from status page
POST   /api/v1/status-page/incidents          — Create public incident
PUT    /api/v1/status-page/incidents/:id      — Update incident status
POST   /api/v1/status-page/incidents/:id/update — Add incident update
```

### Public Status Response Shape

```json
{
  "campus": "Main Campus",
  "overall_status": "degraded",
  "last_updated": "2026-06-05T14:22:00Z",
  "groups": [
    {
      "name": "Core Infrastructure",
      "services": [
        {
          "name": "Internet Gateway",
          "status": "operational",
          "uptime_30d": 99.7,
          "response_time_ms": 12
        },
        {
          "name": "Campus DNS",
          "status": "operational",
          "uptime_30d": 99.9
        }
      ]
    },
    {
      "name": "Academic Services",
      "services": [
        {
          "name": "College ERP",
          "status": "operational",
          "uptime_30d": 98.2
        },
        {
          "name": "Moodle LMS",
          "status": "degraded",
          "uptime_30d": 96.5,
          "message": "Slower than usual"
        },
        {
          "name": "Email Server",
          "status": "outage",
          "uptime_30d": 97.1
        }
      ]
    }
  ],
  "active_incidents": [
    {
      "id": 42,
      "title": "Email Server Maintenance",
      "severity": "warning",
      "status": "identified",
      "started_at": "2026-06-05T13:00:00Z",
      "updates": [
        { "time": "2026-06-05T13:00:00Z", "status": "investigating", "message": "Email server unreachable" },
        { "time": "2026-06-05T13:15:00Z", "status": "identified", "message": "Hard disk failure. Replacing drive." }
      ]
    }
  ],
  "uptime_chart": {
    "days": 90,
    "data": [
      { "date": "2026-06-05", "status": "degraded" },
      { "date": "2026-06-04", "status": "operational" },
      { "date": "2026-06-03", "status": "operational" }
    ]
  }
}
```

---

## Feature 5: Maintenance Windows

### Purpose
College networks have **scheduled downtime** — labs shut down on weekends, servers patched monthly, power cuts for generator testing. Without maintenance windows, every planned shutdown floods the system with false alerts.

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Sunday Lab Shutdown"
    description TEXT,
    scope_type TEXT NOT NULL,              -- 'device', 'location', 'subnet', 'device_group', 'global'
    scope_value TEXT NOT NULL,             -- device ID, location ID, subnet CIDR, or "*"
    schedule_type TEXT NOT NULL,           -- 'once', 'recurring'
    -- For one-time windows:
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    -- For recurring windows:
    recurrence_rule TEXT,                  -- iCal RRULE: "FREQ=WEEKLY;BYDAY=SU"
    recurrence_start_time TEXT,            -- "06:00" (HH:MM local)
    recurrence_end_time TEXT,              -- "10:00" (HH:MM local)
    recurrence_timezone TEXT DEFAULT 'Asia/Kolkata',
    -- Behavior:
    suppress_alerts BOOLEAN DEFAULT true,     -- Don't fire alerts during this window
    suppress_notifications BOOLEAN DEFAULT true,  -- Don't send notifications
    show_maintenance_status BOOLEAN DEFAULT true, -- Show "Maintenance" instead of "Down"
    -- Metadata:
    created_by INTEGER,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_maint_scope ON maintenance_windows(scope_type, scope_value);
CREATE INDEX IF NOT EXISTS idx_maint_schedule ON maintenance_windows(schedule_type, start_time, end_time);
```

### API Endpoints

```
GET    /api/v1/maintenance                    — List all maintenance windows
GET    /api/v1/maintenance/active             — Currently active windows
POST   /api/v1/maintenance                    — Create maintenance window
PUT    /api/v1/maintenance/:id                — Update window
DELETE /api/v1/maintenance/:id                — Delete window
POST   /api/v1/maintenance/:id/toggle         — Enable/disable
GET    /api/v1/maintenance/calendar           — Calendar view (month/week)
GET    /api/v1/devices/:id/maintenance        — Maintenance windows for a device
```

---

## Feature 6: Contact Management & Escalation Policies

### Purpose
In a college, different people are responsible for different network segments. When a lab switch fails, the **lab admin** should be notified first — not the college principal. If the lab admin doesn't respond in 15 minutes, it escalates to the HOD, then to the IT Director. This feature makes sure the **right person** gets notified at the **right time** via the **right channel**.

### Database Schema

```sql
-- ── Contact directory ───────────────────────────────────────────
CREATE TABLE IF NOT EXISTS contacts (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Mr. Rajesh Sharma"
    designation TEXT,                      -- "System Administrator"
    department TEXT,                       -- "IT Department"
    email TEXT,
    phone TEXT,                            -- "+91-9876543210"
    telegram_chat_id TEXT,                 -- Telegram user/group chat ID
    whatsapp_number TEXT,
    preferred_channel TEXT DEFAULT 'email', -- 'email', 'telegram', 'whatsapp', 'sms'
    notification_enabled BOOLEAN DEFAULT true,
    quiet_hours_start TEXT,                -- "22:00" — don't notify after this
    quiet_hours_end TEXT,                  -- "07:00" — resume notifications
    user_id BIGINT,                       -- Link to users table (if they have login)
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- ── Device-to-contact assignment ────────────────────────────────
CREATE TABLE IF NOT EXISTS device_contacts (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT,                     -- Specific device (NULL = location-level)
    location_id BIGINT,                   -- Location-level contact (NULL = device-level)
    contact_id BIGINT NOT NULL,
    role TEXT DEFAULT 'primary',            -- 'primary', 'secondary', 'escalation', 'viewer'
    notify_on TEXT DEFAULT 'critical,warning', -- Comma-separated severities
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE,
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_dc_device ON device_contacts(device_id);
CREATE INDEX IF NOT EXISTS idx_dc_location ON device_contacts(location_id);

-- ── Escalation policies ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS escalation_policies (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Default IT Escalation"
    description TEXT,
    scope_type TEXT DEFAULT 'global',      -- 'global', 'location', 'device_category'
    scope_value TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS escalation_steps (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL,
    step_order INTEGER NOT NULL,           -- 1, 2, 3...
    contact_id BIGINT NOT NULL,
    delay_minutes INTEGER NOT NULL,        -- Wait this long before escalating
    notify_via TEXT DEFAULT 'preferred',   -- 'preferred', 'email', 'telegram', 'sms', 'all'
    repeat_count INTEGER DEFAULT 1,        -- Notify this many times at this step
    repeat_interval_minutes INTEGER DEFAULT 5,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (policy_id) REFERENCES escalation_policies(id) ON DELETE CASCADE,
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_esc_steps_policy ON escalation_steps(policy_id, step_order);

-- ── On-call rotation ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS oncall_schedules (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Weekend On-Call Rotation"
    policy_id BIGINT NOT NULL,            -- Which escalation policy to use
    rotation_type TEXT DEFAULT 'weekly',   -- 'daily', 'weekly', 'custom'
    participants JSONB NOT NULL,        -- JSON array of contact IDs in rotation order
    current_index INTEGER DEFAULT 0,       -- Who's currently on call
    rotation_time TEXT DEFAULT '09:00',    -- When rotation switches
    timezone TEXT DEFAULT 'Asia/Kolkata',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (policy_id) REFERENCES escalation_policies(id) ON DELETE CASCADE
);
```

### Escalation Flow

```
Alert fires (Critical) for device in "CS Lab 1"
    │
    ▼
Step 1 (0 min): Notify Mr. Sharma (Lab Admin) via Telegram
    │ ... waits 15 minutes, not acknowledged ...
    ▼
Step 2 (15 min): Notify Mr. Sharma again + Dr. Patel (HOD CS) via Telegram + Email
    │ ... waits 30 minutes, not acknowledged ...
    ▼
Step 3 (45 min): Notify IT Director via SMS + Email + Telegram
    │ ... waits 60 minutes, not acknowledged ...
    ▼
Step 4 (105 min): Notify Principal via SMS
```

### API Endpoints

```
# Contacts
GET    /api/v1/contacts                       — List all contacts
POST   /api/v1/contacts                       — Create contact
PUT    /api/v1/contacts/:id                   — Update contact
DELETE /api/v1/contacts/:id                   — Delete contact
GET    /api/v1/contacts/:id/devices           — Devices assigned to this contact

# Device-Contact Assignment
POST   /api/v1/devices/:id/contacts           — Assign contact to device
DELETE /api/v1/devices/:id/contacts/:cid      — Remove contact from device
POST   /api/v1/locations/:id/contacts         — Assign contact to location

# Escalation Policies
GET    /api/v1/escalation-policies            — List policies
POST   /api/v1/escalation-policies            — Create policy with steps
PUT    /api/v1/escalation-policies/:id        — Update policy
DELETE /api/v1/escalation-policies/:id        — Delete policy

# On-Call
GET    /api/v1/oncall                         — Current on-call person
GET    /api/v1/oncall/schedule                — Full rotation schedule
PUT    /api/v1/oncall/:id/override            — Temporary override (swap shifts)
```

---

## Feature 7: Incident Management

### Purpose
Alerts tell you *something is wrong*. Incidents track *what you're doing about it*. In a college setting, the administration needs proof that IT responded quickly, what the root cause was, and how long the outage lasted. Incidents provide the **accountability trail** — from detection to resolution — with SLA compliance tracking.

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS incidents (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    severity TEXT NOT NULL,                -- 'critical', 'major', 'minor', 'info'
    status TEXT DEFAULT 'open',            -- 'open', 'investigating', 'identified',
                                           -- 'fixing', 'monitoring', 'resolved', 'closed'
    root_cause TEXT,                        -- Free text root cause analysis
    root_cause_category TEXT,              -- 'hardware', 'software', 'network', 'power',
                                           -- 'configuration', 'external', 'unknown'
    resolution TEXT,                        -- What was done to fix it
    source TEXT DEFAULT 'auto',            -- 'auto' (from alert), 'manual'
    source_alert_id BIGINT,               -- Alert that created this incident
    assigned_to INTEGER,                   -- Contact responsible for resolution
    location_id BIGINT,                   -- Primary affected location
    impact_description TEXT,               -- "40 workstations in CS Lab 1 offline"
    affected_device_count INTEGER DEFAULT 0,
    started_at TIMESTAMPTZ DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    duration_seconds INTEGER,              -- Total time from start to resolution
    sla_breached BOOLEAN DEFAULT false,        -- 1 if resolution time exceeded SLA
    created_by INTEGER,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (source_alert_id) REFERENCES alerts(id),
    FOREIGN KEY (assigned_to) REFERENCES contacts(id),
    FOREIGN KEY (location_id) REFERENCES locations(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_started ON incidents(started_at);
CREATE INDEX IF NOT EXISTS idx_incidents_location ON incidents(location_id);

-- Devices affected by an incident
CREATE TABLE IF NOT EXISTS incident_devices (
    incident_id BIGINT NOT NULL,
    device_id BIGINT NOT NULL,
    PRIMARY KEY (incident_id, device_id),
    FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Timeline entries (notes, status changes, actions)
CREATE TABLE IF NOT EXISTS incident_timeline (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL,
    entry_type TEXT NOT NULL,              -- 'status_change', 'note', 'assignment',
                                           -- 'alert_linked', 'device_recovered', 'escalation'
    old_value TEXT,                         -- Previous status (for status changes)
    new_value TEXT,                         -- New status
    message TEXT NOT NULL,                 -- Human-readable entry
    author TEXT,                           -- "user:admin", "system", "escalation:step2"
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_inc_timeline ON incident_timeline(incident_id, created_at);

-- SLA definitions per severity
CREATE TABLE IF NOT EXISTS sla_definitions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Default Campus SLA"
    severity TEXT NOT NULL UNIQUE,         -- 'critical', 'major', 'minor'
    response_time_minutes INTEGER NOT NULL, -- Acknowledge within this time
    resolution_time_minutes INTEGER NOT NULL, -- Resolve within this time
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Seed default SLA
-- INSERT INTO sla_definitions (name, severity, response_time_minutes, resolution_time_minutes)
-- VALUES ('Critical SLA', 'critical', 15, 120),
--        ('Major SLA', 'major', 30, 480),
--        ('Minor SLA', 'minor', 60, 1440);
```

### API Endpoints

```
GET    /api/v1/incidents                      — List incidents (filter by status, severity, location)
GET    /api/v1/incidents/:id                  — Incident detail with timeline
POST   /api/v1/incidents                      — Create incident (manual)
PUT    /api/v1/incidents/:id                  — Update incident
POST   /api/v1/incidents/:id/note             — Add timeline note
POST   /api/v1/incidents/:id/assign           — Assign to contact
POST   /api/v1/incidents/:id/resolve          — Mark resolved with root cause
POST   /api/v1/incidents/:id/close            — Close (after verification)
GET    /api/v1/incidents/stats                 — Incident statistics (MTTR, counts, SLA compliance)
GET    /api/v1/incidents/sla-report            — SLA breach report

# SLA Definitions
GET    /api/v1/sla                            — List SLA definitions
PUT    /api/v1/sla/:id                        — Update SLA thresholds
```

---

## Feature 8: RBAC (Role-Based Access Control)

### Purpose
A college network has multiple stakeholders with different needs. The **IT admin** needs full control, the **CS HOD** only needs to see their department's devices, and a **lab incharge** should only view their lab's status. RBAC ensures each user sees only what's relevant to them and can only perform actions appropriate to their role.

### Role Definitions

```sql
CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,             -- 'super_admin', 'network_admin', 'dept_admin', 'viewer', 'public'
    display_name TEXT NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL,         -- JSON array of permission strings
    is_system BOOLEAN DEFAULT false,           -- System roles cannot be deleted
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Extend users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS contact_id BIGINT REFERENCES contacts(id);
ALTER TABLE users ADD COLUMN IF NOT EXISTS enabled BOOLEAN DEFAULT true;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- Scope restrictions: which locations a user can see
CREATE TABLE IF NOT EXISTS user_scopes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    scope_type TEXT NOT NULL,              -- 'location', 'subnet', 'device_category'
    scope_value TEXT NOT NULL,             -- location_id, subnet CIDR, or category name
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_scopes ON user_scopes(user_id);
```

### Permission Matrix

| Permission | Super Admin | Network Admin | Dept Admin | Viewer |
|---|---|---|---|---|
| `devices.read` | ✅ All | ✅ All | ✅ Own locations | ✅ Own locations |
| `devices.write` | ✅ | ✅ | ❌ | ❌ |
| `devices.delete` | ✅ | ✅ | ❌ | ❌ |
| `alerts.read` | ✅ All | ✅ All | ✅ Own locations | ✅ Own locations |
| `alerts.acknowledge` | ✅ | ✅ | ✅ Own | ❌ |
| `alerts.resolve` | ✅ | ✅ | ❌ | ❌ |
| `alert_rules.write` | ✅ | ✅ | ❌ | ❌ |
| `incidents.write` | ✅ | ✅ | ✅ (notes only) | ❌ |
| `maintenance.write` | ✅ | ✅ | ❌ | ❌ |
| `contacts.write` | ✅ | ✅ | ❌ | ❌ |
| `reports.read` | ✅ All | ✅ All | ✅ Own locations | ❌ |
| `settings.write` | ✅ | ❌ | ❌ | ❌ |
| `users.manage` | ✅ | ❌ | ❌ | ❌ |
| `import.execute` | ✅ | ✅ | ❌ | ❌ |
| `discovery.execute` | ✅ | ✅ | ❌ | ❌ |
| `capture.execute` | ✅ | ✅ | ❌ | ❌ |
| `status_page.manage` | ✅ | ✅ | ❌ | ❌ |
| `system.monitoring` | ✅ | ✅ | ❌ | ❌ |
| `system.logs` | ✅ | ❌ | ❌ | ❌ |

### Realtime Authorization

RBAC applies to WebSocket traffic as strictly as REST traffic:
- Each WebSocket connection is authenticated during upgrade and stores the user's role and scope claims in connection context.
- Broadcasts are filtered per recipient. `metric:update`, `device:status`, `alert:triggered`, `flow:update`, and `capture:packet` are delivered only when the user has permission for the related device/location.
- Admin-only events (`system:*`, audit-log updates, discovery progress, import progress) are sent only to users with the matching permission.
- Scope changes invalidate active WebSocket authorization state; the server either refreshes the connection claims or disconnects the client and forces reconnect.
- Integration tests must verify that a scoped user cannot learn unauthorized device IDs through realtime events.

### API Endpoints

```
# User Management
GET    /api/v1/users                          — List users
POST   /api/v1/users                          — Create user
PUT    /api/v1/users/:id                      — Update user
DELETE /api/v1/users/:id                      — Delete user
PUT    /api/v1/users/:id/role                 — Change role
PUT    /api/v1/users/:id/scopes               — Set location scopes

# Roles
GET    /api/v1/roles                          — List roles with permissions
POST   /api/v1/roles                          — Create custom role
PUT    /api/v1/roles/:id                      — Update role permissions
```

---

## Feature 9: Notification Channels (Telegram, WhatsApp, SMS)

### Purpose
In an Indian college environment, **email alone is insufficient** for urgent alerts — staff may not check email for hours. Telegram is ubiquitous and free, WhatsApp is universal, and SMS works even without internet. This feature adds India-relevant notification channels with interactive acknowledge/resolve capabilities.

### 9A: Telegram Bot

#### Setup & Configuration

| Variable | Description |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Bot token from @BotFather |
| `TELEGRAM_DEFAULT_CHAT_ID` | Default group chat for alerts |

#### Bot Commands

```
/status              — Current network status summary
/device <name|ip>    — Status of a specific device
/ack <alert_id>      — Acknowledge an alert
/resolve <alert_id>  — Resolve an alert
/oncall              — Who's currently on call
/incidents           — Active incidents
/help                — Command reference
```

#### Alert Message Format

```
🔴 CRITICAL ALERT

📍 Location: CS Lab 1, Ground Floor, CS Department
🖥  Device: Lab1-Switch (10.2.1.254)
📋 Category: Switch
⏰ Time: 14:22 IST, June 5, 2026
📝 Message: Device is unreachable (ping timeout)
🔗 Affected: 40 dependent devices

👤 Assigned: Mr. Sharma (Lab Admin)
⏳ Acknowledge within 15 min or escalates to Dr. Patel

[✅ Acknowledge]  [🔍 View Details]
```

The `[✅ Acknowledge]` button is an inline keyboard button that triggers `/ack <id>`.

### 9B: WhatsApp Business API

Uses WhatsApp Business API (requires Meta Business verification) or third-party gateways like Twilio, Gupshup.

| Variable | Description |
|---|---|
| `WHATSAPP_API_URL` | API endpoint |
| `WHATSAPP_API_TOKEN` | Auth token |
| `WHATSAPP_SENDER_NUMBER` | Registered sender number |

### 9C: SMS Gateway

For critical alerts through an SMS provider API. This still requires internet access unless the optional GSM modem/on-premise gateway path is implemented later.

| Variable | Description |
|---|---|
| `SMS_GATEWAY_URL` | SMS API endpoint (MSG91, Twilio, TextLocal) |
| `SMS_API_KEY` | API key |
| `SMS_SENDER_ID` | Registered sender ID |

### Database Schema

```sql
-- Notification delivery log (tracks every notification attempt)
CREATE TABLE IF NOT EXISTS notification_log (
    id BIGSERIAL PRIMARY KEY,
    alert_id BIGINT,
    incident_id BIGINT,
    contact_id BIGINT NOT NULL,
    channel_type TEXT NOT NULL,            -- 'email', 'telegram', 'whatsapp', 'sms', 'in_app'
    recipient TEXT NOT NULL,               -- Email address, phone number, chat ID
    message_preview TEXT,                  -- First 200 chars of message
    status TEXT NOT NULL,                  -- 'queued', 'sent', 'delivered', 'failed', 'read'
    external_id TEXT,                      -- Provider's message ID
    error_message TEXT,
    attempt_count INTEGER DEFAULT 1,
    escalation_step INTEGER,               -- Which escalation step triggered this
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (alert_id) REFERENCES alerts(id),
    FOREIGN KEY (incident_id) REFERENCES incidents(id),
    FOREIGN KEY (contact_id) REFERENCES contacts(id)
);
CREATE INDEX IF NOT EXISTS idx_notif_log_alert ON notification_log(alert_id);
CREATE INDEX IF NOT EXISTS idx_notif_log_contact ON notification_log(contact_id);
CREATE INDEX IF NOT EXISTS idx_notif_log_status ON notification_log(status);
```

---

## Feature 10: Reporting Engine

### Purpose
College IT departments must justify their budget and demonstrate reliability to management. Automated reports — emailed as PDF every Monday morning — show per-building uptime, incident counts, SLA compliance, and ISP performance. No manual work, no Excel spreadsheets, just professional reports delivered to the right inbox on schedule.

### Report Types

| Report | Audience | Frequency | Content |
|---|---|---|---|
| **Daily Health Summary** | IT Team | Daily 8 AM | Overnight issues, current status, devices needing attention |
| **Weekly Network Report** | IT Head | Monday 9 AM | Uptime %, incident count, top-5 worst devices, trend |
| **Monthly SLA Report** | Management | 1st of month | Per-building uptime, SLA compliance, MTTR, incident breakdown |
| **ISP Performance Report** | IT Head | Weekly | ISP link latency, packet loss, jitter, downtime comparison |
| **Incident Report** | HOD/Director | On-demand | Single incident deep-dive with timeline, root cause, duration |
| **Department Report** | Department HOD | Monthly | Devices in their department, uptime, issues |

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS scheduled_reports (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Weekly IT Report"
    report_type TEXT NOT NULL,             -- 'health_summary', 'sla', 'incident', 'isp', 'department'
    format TEXT DEFAULT 'pdf',             -- 'pdf', 'csv', 'html'
    schedule_cron TEXT NOT NULL,           -- "0 9 * * 1" (Monday 9 AM)
    timezone TEXT DEFAULT 'Asia/Kolkata',
    scope_type TEXT DEFAULT 'global',      -- 'global', 'location', 'subnet'
    scope_value TEXT,                      -- location_id, subnet CIDR
    recipients JSONB NOT NULL,         -- JSON array of contact IDs or email addresses
    include_charts BOOLEAN DEFAULT true,
    lookback_period TEXT DEFAULT '7d',     -- "1d", "7d", "30d", "custom"
    custom_from TIMESTAMPTZ,
    custom_to TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_run_status TEXT,                  -- 'success', 'failed'
    enabled BOOLEAN DEFAULT true,
    created_by INTEGER,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_sched_reports_cron ON scheduled_reports(enabled, schedule_cron);

-- Generated report archive
CREATE TABLE IF NOT EXISTS generated_reports (
    id BIGSERIAL PRIMARY KEY,
    scheduled_report_id BIGINT,           -- NULL for on-demand reports
    report_type TEXT NOT NULL,
    title TEXT NOT NULL,
    format TEXT NOT NULL,
    file_path TEXT NOT NULL,               -- Path to generated file
    file_size_bytes INTEGER,
    scope_description TEXT,                -- "CS Department, June 2026"
    period_from TIMESTAMPTZ,
    period_to TIMESTAMPTZ,
    recipients TEXT,                        -- Who it was emailed to
    generated_by TEXT,                     -- "scheduler" or "user:admin"
    generated_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (scheduled_report_id) REFERENCES scheduled_reports(id) ON DELETE SET NULL
);
```

### API Endpoints

```
# On-demand reports
POST   /api/v1/reports/generate               — Generate a report now
       body: { "type": "sla", "format": "pdf", "scope_type": "location",
               "scope_value": "5", "from": "2026-06-01", "to": "2026-06-30" }

GET    /api/v1/reports/generated               — List generated reports
GET    /api/v1/reports/generated/:id/download  — Download report file

# Scheduled reports
GET    /api/v1/reports/scheduled               — List scheduled reports
POST   /api/v1/reports/scheduled               — Create scheduled report
PUT    /api/v1/reports/scheduled/:id           — Update schedule
DELETE /api/v1/reports/scheduled/:id           — Delete schedule
POST   /api/v1/reports/scheduled/:id/run       — Trigger manually now

# Report data APIs (raw data for frontend charts)
GET    /api/v1/reports/sla                     — SLA compliance data
       ?from=ISO8601&to=ISO8601&location_id=5
GET    /api/v1/reports/mttr                    — Mean Time to Resolve by severity/location
GET    /api/v1/reports/availability            — Per-device/location availability %
GET    /api/v1/reports/isp                     — ISP link performance data
GET    /api/v1/reports/top-offenders           — Worst-performing devices
```

### Monthly SLA Report Content (PDF)

```
┌──────────────────────────────────────────────────────┐
│          🌐 Rayavriti NetMonitor                      │
│       Monthly Network Report — June 2026              │
│           Main Campus                                 │
├──────────────────────────────────────────────────────┤
│                                                       │
│  EXECUTIVE SUMMARY                                    │
│  ├ Overall Uptime: 99.2%                              │
│  ├ Total Incidents: 14                                │
│  ├ Critical Incidents: 2                              │
│  ├ Avg Resolution Time: 22 minutes                    │
│  └ SLA Compliance: 92.8%                              │
│                                                       │
│  PER-BUILDING UPTIME                                  │
│  ┌────────────────────┬────────┬──────────┐           │
│  │ Building            │ Uptime │ Incidents│           │
│  ├────────────────────┼────────┼──────────┤           │
│  │ Admin Block         │ 99.8%  │    1     │           │
│  │ CS Department       │ 98.1%  │    5     │           │
│  │ Library             │ 99.9%  │    0     │           │
│  │ Electrical Dept     │ 97.2%  │    4     │           │
│  │ Hostel Block A      │ 99.5%  │    2     │           │
│  └────────────────────┴────────┴──────────┘           │
│                                                       │
│  [Uptime Trend Chart — 30 days]                       │
│  [Incident Timeline Chart]                            │
│  [Top 5 Problematic Devices]                          │
│  [ISP Link Performance]                               │
│                                                       │
└──────────────────────────────────────────────────────┘
```

---

## Feature 11: ISP Link Monitoring

### Purpose
Dedicated, continuous monitoring of the college's internet connection(s) — separate from regular device monitoring. Provides proof for ISP SLA negotiations.

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS isp_links (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,                    -- "Primary ISP (Jio Fiber)"
    provider TEXT NOT NULL,                -- "Jio", "Airtel", "BSNL"
    circuit_id TEXT,                       -- ISP circuit/account ID
    bandwidth_mbps INTEGER,                -- Contracted bandwidth
    gateway_ip TEXT NOT NULL,              -- ISP gateway IP
    sla_uptime_percent REAL,               -- Contracted uptime SLA (e.g., 99.5)
    cost_monthly REAL,                     -- Monthly cost (for ROI calculations)
    contract_start DATE,
    contract_end DATE,
    monitoring_interval_seconds INTEGER DEFAULT 10,  -- More frequent than regular devices
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS isp_metrics (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL,
    latency_ms REAL,
    jitter_ms REAL,
    packet_loss_percent REAL,
    download_speed_mbps REAL,              -- Periodic speed test (optional)
    upload_speed_mbps REAL,
    status TEXT NOT NULL,                  -- 'up', 'degraded', 'down'
    target_ip TEXT,                        -- What was pinged (gateway, 8.8.8.8, etc.)
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (link_id) REFERENCES isp_links(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_isp_metrics_link ON isp_metrics(link_id, created_at);
```

### API Endpoints

```
GET    /api/v1/isp-links                      — List ISP links
POST   /api/v1/isp-links                      — Add ISP link
PUT    /api/v1/isp-links/:id                  — Update link
DELETE /api/v1/isp-links/:id                  — Delete link
GET    /api/v1/isp-links/:id/metrics          — Historical metrics
GET    /api/v1/isp-links/:id/sla              — SLA compliance report
GET    /api/v1/isp-links/comparison            — Compare multiple ISP links side-by-side
```

---

## Feature 12: College Service Templates

Pre-built monitoring templates for common college infrastructure. Auto-creates devices + sensors + alert rules for each service type.

```
POST /api/v1/service-templates/apply
{
  "template": "college_erp",
  "host": "erp.college.edu",
  "location_id": 3,
  "contact_id": 7
}
```

### Available Templates

| Template | Checks Created |
|---|---|
| `college_erp` | HTTP 200 check, login page keyword, response time threshold, SSL expiry |
| `moodle_lms` | HTTP check, `/login/index.php` reachable, response time |
| `email_server` | SMTP (25), IMAP (143), IMAPS (993), POP3S (995) port checks |
| `dns_server` | DNS query resolution test (resolve known domain), port 53 |
| `radius_ldap` | LDAP port (389/636), RADIUS port (1812/1813) |
| `proxy_server` | HTTP through proxy, proxy port check |
| `cctv_nvr` | HTTP management interface, RTSP port (554) |
| `biometric_server` | TCP port check, HTTP management interface |
| `ups_monitoring` | SNMP UPS MIB (battery status, load, input voltage) |
| `printer_network` | TCP 9100 (raw), TCP 631 (IPP), SNMP printer MIB |
| `wifi_controller` | HTTP management, SNMP AP count, connected client count |
| `file_server` | SMB (445), NFS (2049), HTTP file browser |
| `database_server` | MySQL (3306), PostgreSQL (5432), MongoDB (27017) port check |

### API Endpoints

```
GET    /api/v1/service-templates              — List available templates
GET    /api/v1/service-templates/:name        — Template details (what it creates)
POST   /api/v1/service-templates/apply        — Apply template to a host
```

---

## Environment Variables Summary

All new configuration for Phase 2 features:

| Variable | Default | Feature | Description |
|---|---|---|---|
| `TELEGRAM_BOT_TOKEN` | (none) | Notifications | Telegram Bot API token from @BotFather |
| `TELEGRAM_DEFAULT_CHAT_ID` | (none) | Notifications | Default group chat for alerts |
| `TELEGRAM_MODE` | `polling` | Notifications | `polling` (behind NAT) or `webhook` (public IP) |
| `WHATSAPP_API_URL` | (none) | Notifications | WhatsApp Business API endpoint |
| `WHATSAPP_API_TOKEN` | (none) | Notifications | WhatsApp API auth token |
| `WHATSAPP_SENDER_NUMBER` | (none) | Notifications | Registered sender phone number |
| `SMS_GATEWAY_URL` | (none) | Notifications | SMS provider API endpoint |
| `SMS_API_KEY` | (none) | Notifications | SMS provider API key |
| `SMS_SENDER_ID` | (none) | Notifications | Registered SMS sender ID |
| `REPORT_OUTPUT_DIR` | `./data/reports` | Reports | Directory for generated PDF/CSV files |
| `REPORT_SMTP_HOST` | (none) | Reports | SMTP server for emailing reports |
| `REPORT_SMTP_PORT` | `587` | Reports | SMTP port |
| `REPORT_SMTP_USER` | (none) | Reports | SMTP username |
| `REPORT_SMTP_PASS` | (none) | Reports | SMTP password |
| `REPORT_FROM_EMAIL` | `netmonitor@college.edu` | Reports | Sender email for reports |
| `ISP_MONITOR_INTERVAL` | `10` | ISP Monitor | Seconds between ISP link checks |
| `ISP_EXTERNAL_TARGETS` | `8.8.8.8,1.1.1.1` | ISP Monitor | Comma-separated IPs to ping through ISP |
| `STATUS_PAGE_TITLE` | `Campus Network Status` | Status Page | Title on public status page |
| `STATUS_PAGE_LOGO_URL` | (none) | Status Page | College logo URL for status page |
| `DISCOVERY_MAX_CONCURRENT` | `64` | Auto-Discovery | Max parallel probes during subnet scan |
| `DISCOVERY_TIMEOUT_MS` | `2000` | Auto-Discovery | Per-host probe timeout |
| `DEFAULT_TIMEZONE` | `Asia/Kolkata` | Global | Default timezone for schedules and reports |

---

## Implementation Priority & Release Milestones

The original feature list is too large for a single reliable release. Ship Phase 2 as incremental releases, each with its own migration, UI, tests, and rollback notes.

| Release | Scope | Estimated Effort | Exit Criteria |
|---|---|---|---|
| **2.1 Campus Inventory MVP** | Location hierarchy, subnet registry, bulk CSV import, device metadata fields | 2-3 weeks | 100+ devices can be imported, assigned to locations, searched, and shown in the existing device views |
| **2.2 Dependency & Maintenance Controls** | Parent-device topology, root-cause alert suppression, maintenance windows | 2-3 weeks | Parent outage creates one root-cause alert, suppressed child alerts are auditable, maintenance suppresses alerts correctly |
| **2.3 Contacts + Telegram Notifications** | Contacts, device/location assignments, default escalation policy, Telegram long polling | 2 weeks | Critical alert reaches the right contact, acknowledge/resolve commands update the dashboard |
| **2.4 RBAC Hardening** | Roles, user scopes, API filtering, WebSocket event filtering, frontend permission handling | 2-3 weeks | Scoped users cannot read unauthorized devices through REST, WebSocket, reports, or dashboard bootstrap payloads |
| **2.5 Status + Incident Layer** | Public status page, incident lifecycle, SLA definitions, timeline | 3-4 weeks | Public status reflects configured services, critical alerts can create incidents, SLA breach metrics are correct |
| **2.6 Reporting & ISP Visibility** | Scheduled reports, generated report archive, ISP link monitoring, service templates | 3-4 weeks | Reports generate from PostgreSQL/Timescale data and ISP metrics can support SLA review |
| **Later Optional** | Auto-discovery, WhatsApp, SMS provider APIs, GSM modem gateway | 2-4 weeks | Provider-specific credentials, rate limits, and operational constraints are validated |

**Total realistic effort: ~16-23 weeks for one developer**, depending on frontend polish, production hardening, and notification-provider availability. With 2-3 developers, backend, frontend, and QA streams can overlap after Release 2.1.

---

## Verification Plan

### Automated Tests

```bash
cd backend && go test ./internal/campus/... ./internal/contacts/... ./internal/incidents/... \
   ./internal/rbac/... ./internal/notifications/... ./internal/reports/... \
   ./internal/importer/... ./internal/statuspage/... ./internal/maintenance/... -v -count=1
```

#### Unit Tests by Feature

| Feature | Test Coverage |
|---|---|
| Location Hierarchy | Tree CRUD, move operations, circular dependency prevention, recursive device counts, status aggregation |
| Dependency Tree | Parent chain walk, suppression logic, root-cause detection, cascading outage grouping, multi-level chains |
| Bulk Import | CSV parsing, field validation (IP format, required fields), duplicate detection, location code resolution, transactional rollback on error |
| Auto-Discovery | Subnet CIDR expansion, OUI lookup, device categorization heuristics, SNMP probe mocking |
| Status Page | Service aggregation logic (`any_down`, `all_down`, `majority`), uptime calculation, incident status transitions |
| Maintenance Windows | Recurring RRULE evaluation, timezone handling, overlap detection, device-in-window check |
| Contacts & Escalation | Multi-step escalation timing, on-call rotation index advance, quiet hours filtering, notification channel selection |
| Incident Management | Auto-creation from alerts, timeline append, SLA breach detection, duration calculation, status transitions |
| RBAC | Permission check for all 18 permission types × 5 roles, scope filtering (location-based query filtering), 403 on unauthorized access |
| Telegram Bot | Message formatting, inline keyboard JSON, command parsing (`/ack`, `/status`, `/device`), error handling |
| Reporting Engine | Data aggregation queries, PDF generation (file output), cron schedule parsing, email delivery mock |
| ISP Monitoring | Jitter calculation, packet loss tracking, SLA compliance percentage, multi-target ping results |

### Integration Tests

```bash
cd backend && go test ./integration/phase2/... -v -tags integration
```

- Full onboarding flow: Create locations → import 100 devices via CSV → verify tree structure → set dependencies
- Alert suppression end-to-end: Create parent-child chain → parent goes down → verify child alerts suppressed
- Escalation end-to-end: Alert fires → Step 1 notification sent → timeout → Step 2 escalation fires
- Incident lifecycle: Alert → auto-incident → assign → add notes → resolve → verify SLA metrics
- RBAC scoping: Create 3 users with different roles → verify each sees only permitted data across all endpoints

### Manual Verification

1. **Onboarding flow**: Import 100+ devices via CSV → verify location assignment → set parents → verify topology view
2. **Alert suppression**: Take a switch offline → verify only 1 root-cause alert fires (not 40 device alerts) → verify 40 suppressed alerts logged
3. **Escalation**: Trigger critical alert → verify Telegram notification to primary contact → wait 15 min → verify escalation to secondary
4. **Status page**: Visit `/status` without login → verify all configured services show correct status → verify auto-refresh
5. **Maintenance**: Create recurring Sunday window → take device offline on Sunday → verify no alerts → verify "Maintenance" status label
6. **RBAC**: Login as `dept_admin` → verify can only see own department's devices → verify 403 on other endpoints
7. **Incident**: Alert auto-creates incident → add notes → assign to contact → resolve with root cause → verify SLA compliance calculation
8. **Reports**: Generate monthly SLA PDF → verify per-building breakdown → verify auto-email delivery to configured recipients
9. **Telegram bot**: Send `/status` → verify response → `/ack 142` → verify alert acknowledged in dashboard
10. **ISP Monitor**: Add 2 ISP links → verify metrics collection every 10s → verify comparison view → verify SLA calculation

### Regression Checklist

- [ ] All Phase 1 features still work (devices, metrics, alerts, flows, capture, insights)
- [ ] Location hierarchy: create building → floor → room → assign devices
- [ ] Bulk CSV import: 100+ devices imported in single operation
- [ ] Dependency tree: parent-child relationships visualized correctly
- [ ] Alert suppression: child device alerts suppressed when parent is down
- [ ] Root-cause alerts: single alert with affected device count
- [ ] Public status page: loads without authentication, shows correct service status
- [ ] Status page incidents: create → update → resolve lifecycle
- [ ] Maintenance windows: recurring schedule suppresses alerts correctly
- [ ] Maintenance windows: device shows "Maintenance" status (not "Down") during window
- [ ] Contacts: create contact with Telegram chat ID, assign to device
- [ ] Escalation: multi-step escalation fires on correct timing
- [ ] On-call rotation: current on-call person rotates on schedule
- [ ] Quiet hours: no notifications sent during contact's quiet hours
- [ ] Incident auto-creation: critical alert auto-creates incident with linked devices
- [ ] Incident timeline: all status changes and notes recorded chronologically
- [ ] SLA tracking: breach detected when resolution time exceeds threshold
- [ ] RBAC: `super_admin` has full access, `dept_admin` scoped to locations, `viewer` read-only
- [ ] Telegram bot: `/status`, `/ack`, `/resolve` commands work correctly
- [ ] Scheduled reports: cron-triggered PDF generated and emailed
- [ ] ISP monitoring: metrics collected at configured interval, SLA calculated
- [ ] Auto-discovery: subnet scan finds live devices, results stored for approval
- [ ] Service templates: applying template creates device + sensors + alert rules
- [ ] Database migration: existing Phase 1 data preserved after Phase 2 schema extensions
