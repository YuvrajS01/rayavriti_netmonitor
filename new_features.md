# Rayavriti NetMonitor New Feature Roadmap

Purpose: brainstorm major missing and must-have features for making Rayavriti NetMonitor a one-stop network monitoring solution for small organizations such as colleges, schools, offices, hostels, labs, and small campuses.

## Must-Have Features Before Serious Production Use

### 1. Strong Identity, Access, and Account Security

- SSO login with Google Workspace, Microsoft Entra ID, LDAP, SAML, and OIDC.
- Mandatory MFA/2FA for admins and network operators.
- Role-based access control enforced on every endpoint and UI action.
- Department-level and location-level access boundaries.
- Session management page showing active sessions, IP, device, and last activity.
- Force logout for disabled users, password changes, role changes, and suspected compromise.
- Password policy: minimum length, complexity, breach checks, expiry option, and account lockout.
- User invite flow with temporary links instead of manually creating passwords.
- API key scopes, expiry, rotation, last-used tracking, and revoke controls.

### 2. Secure Credential Management

- Encrypted credential vault for SNMP, SSH, API, controller, firewall, and ISP credentials.
- SNMPv3 support with auth/privacy protocols.
- Per-site and per-device credential inheritance.
- Credential rotation workflow.
- Secret usage audit trail.
- Integration with external vaults such as HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, or Bitwarden Secrets Manager.
- UI warnings for weak SNMP community strings like `public`.

### 3. Device Discovery and Inventory Management

- Scheduled subnet discovery.
- LLDP/CDP topology discovery for switches, routers, and access points.
- ARP table and MAC address table discovery.
- DHCP lease import.
- DNS reverse lookup integration.
- Device fingerprinting by open ports, SNMP sysObjectID, MAC OUI, HTTP banners, and certificates.
- Device approval queue with bulk approve/reject.
- Duplicate device detection by IP, MAC, serial number, hostname, or asset tag.
- Device lifecycle states: active, planned, maintenance, retired, lost, unmanaged.
- Asset inventory fields: owner, department, warranty, vendor, purchase date, rack, room, floor, serial, support contract.

### 4. Network Topology and Mapping

- Automatic topology map from LLDP/CDP, parent-child links, and switch MAC tables.
- Manual topology editor for missing links.
- Floor/building/campus map views.
- Rack elevation view.
- Dependency map showing upstream/downstream outage impact.
- Root cause highlighting when a parent device or ISP link fails.
- Link utilization overlays.
- VLAN and subnet visualizations.
- Export topology as PDF/PNG.

### 5. Alerting and Incident Management

- Alert deduplication and correlation.
- Alert suppression during maintenance windows.
- Escalation policies with on-call schedules.
- Alert severity rules by device type, department, location, and business hours.
- Incident auto-creation from correlated alerts.
- Incident timeline with notes, ownership, status changes, linked devices, and linked alerts.
- SLA timers for response and resolution.
- MTTA and MTTR reports.
- Alert fatigue dashboard.
- Bulk acknowledge, resolve, assign, and suppress.
- Post-incident review template.

### 6. Notification Channels

- Email notifications.
- SMS gateway integration.
- WhatsApp integration.
- Telegram integration.
- Slack and Microsoft Teams webhooks.
- Voice call escalation for critical outages.
- Web push notifications.
- Per-user notification preferences.
- Quiet hours and escalation override.
- Notification delivery tracking and retry history.
- Test notification button per channel.

### 7. Monitoring Coverage

- Ping and latency monitoring.
- HTTP/HTTPS monitoring with status code, keyword, certificate expiry, TLS version, and response time.
- TCP port monitoring.
- SNMP metrics for CPU, memory, interface traffic, errors, discards, temperature, power supplies, fans, storage, and uptime.
- Wireless controller/AP monitoring.
- Firewall monitoring.
- UPS monitoring.
- CCTV/NVR monitoring.
- DNS, DHCP, RADIUS, LDAP, SMTP, IMAP, VPN, and proxy service checks.
- Printer and biometric attendance device monitoring for college environments.
- Certificate expiry monitoring.
- Public website uptime monitoring from external targets.
- ISP link monitoring with packet loss, latency, jitter, failover status, and speed checks.

### 8. Logs, Flows, and Packet Visibility

- Syslog ingestion and search.
- NetFlow/sFlow/IPFIX collection and dashboards.
- Top talkers, top protocols, top destinations, and bandwidth anomalies.
- Flow retention policies.
- Packet capture approval workflow.
- Time-limited packet capture with quotas.
- Metadata-only packet capture mode by default.
- PCAP export for authorized admins.
- DNS query visibility.
- NAT/firewall log parsing.

### 9. Reporting and Compliance

- Scheduled PDF/CSV reports.
- Email report delivery.
- SLA report by department/location.
- Uptime report by device, service, building, and ISP.
- Bandwidth utilization report.
- Incident report.
- Inventory report.
- Audit log report.
- Security posture report.
- Capacity planning report.
- Custom report builder.
- Report templates for college management and IT teams.

### 10. Backup, Restore, and Disaster Recovery

- One-click configuration backup.
- Scheduled database backups.
- Restore workflow with validation.
- Export/import full configuration.
- Backup encryption.
- Backup health check alerts.
- Disaster recovery documentation.
- Migration assistant from older versions.
- Offline backup download.

## High-Value Features for Colleges and Campuses

### Department and Location Model

- Campus, building, floor, room, lab, hostel, office hierarchy.
- Department ownership for devices and services.
- Department-specific dashboards.
- Department-level uptime and issue reports.
- Location-based alert routing.
- Room/lab equipment inventory.

### Student/Staff Impact Awareness

- Mark services as student-facing, staff-facing, admin-facing, or internal.
- Impact score based on affected buildings, labs, departments, or hostels.
- Public status page for students and staff.
- Subscriber notifications for status page incidents.
- Maintenance announcements.

### Helpdesk Integration

- Ticket creation from incidents.
- Integrations with Jira Service Management, Freshservice, ServiceNow, GLPI, Zammad, or osTicket.
- Link alerts/incidents to tickets.
- Two-way status sync.
- Assignment mapping by location or department.

### ISP and Internet Gateway Management

- Multi-ISP comparison dashboard.
- ISP SLA tracking.
- Circuit inventory with cost, bandwidth, contract dates, account manager, and support contact.
- Failover detection.
- Bandwidth billing reports.
- Internet outage timeline.

### Lab and Classroom Monitoring

- Lab switch health.
- Access point health.
- Smart classroom device checks.
- Projector/AV controller reachability.
- Computer lab uptime.
- Scheduled monitoring profiles by class hours.

## Advanced Features

### AI and Anomaly Detection

- Baseline latency, bandwidth, CPU, and memory per device.
- Detect abnormal traffic spikes.
- Detect unusual device flapping.
- Detect likely root cause.
- Predict capacity exhaustion.
- Suggest alert thresholds based on historical data.
- Summarize incidents automatically.
- Generate plain-English outage explanations for management.
- Recommend remediation steps.

### Automation and Remediation

- Webhook automation.
- Runbook automation.
- Restart service or port action through SSH/API for approved devices.
- Auto-create maintenance windows.
- Auto-disable noisy rules temporarily with approval.
- Config backup before risky changes.
- Approval workflow for destructive actions.

### High Availability and Scale

- Multiple collector nodes.
- Remote collectors for branch campuses.
- Collector health monitoring.
- Leader election for scheduled jobs.
- Sharding polling load across collectors.
- Offline buffering when central server is unreachable.
- Multi-tenant deployment mode.
- Horizontal API scaling.

### Observability of NetMonitor Itself

- Internal health dashboard.
- Database pool metrics.
- Redis metrics.
- Collector queue metrics.
- Scheduler lag.
- WebSocket connection count and dropped events.
- API latency percentiles.
- Error rates by endpoint.
- Self-alerts when NetMonitor is unhealthy.

### Mobile and Field Operations

- Mobile-friendly incident view.
- Push notifications.
- QR code on device/rack to open device page.
- Field notes with photos.
- Offline checklist for technicians.
- Maintenance completion confirmation from mobile.

## Frontend and UX Features

- Global search for device name, IP, MAC, serial, room, building, alert, incident, and contact.
- Command palette for power users.
- Saved filters and saved views.
- Custom dashboards.
- Drag-and-drop dashboard widgets.
- NOC wallboard mode.
- Dark/light theme.
- Keyboard shortcuts.
- Bulk actions for devices and alerts.
- Table column customization.
- CSV import preview with validation errors.
- Rich empty states for first setup.
- Guided onboarding wizard.
- Setup checklist for first deployment.
- Interactive topology editor.
- Accessibility checks and keyboard-only usability.

## Security and Governance Features

- Full audit trail for every sensitive action.
- Tamper-resistant audit storage.
- Exportable audit reports.
- IP allowlist for admin access.
- Admin action approval workflow.
- Packet capture approval workflow.
- Discovery scan approval workflow.
- Per-route permission matrix documentation.
- Security dashboard showing weak credentials, default SNMP, exposed services, expired certs, and stale users.
- Compliance mode for stricter deployments.

## Developer and Operations Features

- OpenAPI/Swagger documentation.
- Versioned public API.
- Webhook API.
- Plugin system for collectors and notification channels.
- Import/export configuration as code.
- Terraform/Ansible deployment examples.
- Helm chart for Kubernetes.
- Systemd deployment guide.
- Offline installer.
- Upgrade checker.
- Migration status UI.
- Feature flags.
- Admin diagnostics bundle with secrets redacted.

## Suggested Feature Priority

### Phase 1: Production Safety

1. Full RBAC enforcement.
2. MFA/SSO.
3. Secret/credential vault.
4. Audit logging for all sensitive actions.
5. Backup/restore.
6. Strong deployment hardening.
7. Query limits and pagination.
8. Packet capture safety controls.

### Phase 2: Core Network Operations

1. SNMPv3 and richer SNMP metrics.
2. Scheduled discovery.
3. LLDP/CDP topology.
4. Incident management.
5. Escalation and on-call.
6. Syslog ingestion.
7. NetFlow/sFlow dashboards.
8. Scheduled reports.

### Phase 3: College-Focused Differentiators

1. Campus/building/floor/room model.
2. Department dashboards.
3. Student/staff public status page.
4. Helpdesk integration.
5. Lab/classroom monitoring templates.
6. ISP contract and SLA tracking.
7. Management-ready reports.

### Phase 4: Advanced Intelligence

1. Anomaly detection.
2. Root cause analysis.
3. Capacity forecasting.
4. Automated remediation.
5. Multi-collector HA.
6. AI incident summaries.

## Best Feature Bets

If the goal is to win small colleges and similar organizations, the strongest differentiators are:

- Zero-touch discovery plus approval queue.
- Campus-aware topology map.
- Department/location dashboards.
- Simple public status page for students and staff.
- ISP SLA and outage reporting.
- WhatsApp/SMS/Telegram notifications.
- Easy PDF reports for management.
- Secure credential vault with SNMPv3.
- Guided setup wizard that gets a college monitoring devices in under one hour.
