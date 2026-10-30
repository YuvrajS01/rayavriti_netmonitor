/**
 * The information architecture: navigation groups, their items, the
 * section tabs inside each item, and the old URLs that now redirect.
 *
 * Layout (sidebar), PageHeader (breadcrumbs), SectionNav (hub tabs),
 * SettingsLayout (side tabs) and the command palette all read from here, so
 * a page is added or renamed in one place.
 */
import type { SessionUser } from './store/session';

export interface NavSection {
  id: string;
  to: string;
  label: string;
  icon: string;
  permission?: string;
  /** Only an exact path match activates it (index sections). */
  end?: boolean;
  /** Extra words the command palette matches on. */
  keywords?: string;
}

export interface NavItem {
  id: string;
  label: string;
  icon: string;
  /** Default target when the item has no sections. */
  to: string;
  end?: boolean;
  permission?: string;
  /** Hub tabs (or settings side tabs). The item shows when any is permitted. */
  sections?: NavSection[];
  /** Deeper pages that aren't sections but still need a breadcrumb. */
  subpages?: Array<{ to: string; label: string }>;
  keywords?: string;
}

export interface NavGroup {
  id: string;
  label: string;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    id: 'monitor',
    label: 'Monitor',
    items: [
      { id: 'overview', label: 'Overview', icon: 'dashboard', to: '/', end: true, keywords: 'dashboard home' },
      {
        id: 'alerts',
        label: 'Alerts & incidents',
        icon: 'notifications',
        to: '/alerts',
        sections: [
          { id: 'alerts', to: '/alerts', label: 'Alerts', icon: 'warning', permission: 'alerts.read', end: true },
          { id: 'incidents', to: '/alerts/incidents', label: 'Incidents', icon: 'crisis_alert', permission: 'incidents.read' },
          { id: 'maintenance', to: '/alerts/maintenance', label: 'Maintenance', icon: 'event_repeat', permission: 'maintenance.read', keywords: 'windows schedule' },
        ],
      },
      {
        id: 'topology',
        label: 'Topology',
        icon: 'hub',
        to: '/topology',
        sections: [
          { id: 'network', to: '/topology', label: 'Network', icon: 'hub', permission: 'devices.read', end: true, keywords: 'map graph' },
          { id: 'campus', to: '/topology/campus', label: 'Campus', icon: 'account_tree', permission: 'locations.read', keywords: 'buildings tree' },
          { id: 'floor-plans', to: '/topology/floor-plans', label: 'Floor plans', icon: 'grid_view', permission: 'locations.read' },
        ],
      },
      { id: 'wall', label: 'Wall display', icon: 'tv', to: '/wall', permission: 'devices.read', keywords: 'noc fullscreen tv' },
    ],
  },
  {
    id: 'inventory',
    label: 'Inventory',
    items: [
      {
        id: 'devices',
        label: 'Devices',
        icon: 'devices',
        to: '/devices',
        permission: 'devices.read',
        subpages: [{ to: '/devices/sensors', label: 'Sensor overview' }],
        keywords: 'cameras biometrics inventory',
      },
      { id: 'isp', label: 'ISP links', icon: 'router', to: '/isp', permission: 'isp.read', keywords: 'wan uplink' },
      { id: 'remote', label: 'Remote sites', icon: 'lan', to: '/remote', permission: 'remote.manage', keywords: 'remote monitoring instances' },
    ],
  },
  {
    id: 'analyze',
    label: 'Analyze',
    items: [
      {
        id: 'flows',
        label: 'Flows & capture',
        icon: 'swap_horiz',
        to: '/flows',
        sections: [
          { id: 'flows', to: '/flows', label: 'Flows', icon: 'swap_horiz', permission: 'flows.read', end: true, keywords: 'netflow top talkers' },
          { id: 'capture', to: '/flows/capture', label: 'Packet capture', icon: 'network_check', permission: 'capture.read', keywords: 'pcap' },
        ],
      },
      { id: 'logs', label: 'Logs', icon: 'receipt_long', to: '/logs', permission: 'system.logs' },
      {
        id: 'reports',
        label: 'Reports',
        icon: 'analytics',
        to: '/reports',
        sections: [
          { id: 'library', to: '/reports/library', label: 'Library', icon: 'description', permission: 'reports.read', keywords: 'generated archive' },
          { id: 'builder', to: '/reports', label: 'Builder', icon: 'analytics', permission: 'reports.read', end: true, keywords: 'sla availability summary' },
          { id: 'scheduled', to: '/reports/scheduled', label: 'Scheduled', icon: 'schedule', permission: 'reports.write' },
        ],
      },
      { id: 'ai-health', label: 'AI health', icon: 'psychology', to: '/ai-health', permission: 'insights.read', keywords: 'insights score risk' },
    ],
  },
  {
    id: 'configure',
    label: 'Configure',
    items: [
      {
        id: 'settings',
        label: 'Settings',
        icon: 'settings',
        to: '/settings',
        sections: [
          { id: 'general', to: '/settings', label: 'General', icon: 'tune', end: true, keywords: 'theme density appearance account' },
          { id: 'users', to: '/settings/users', label: 'Users & roles', icon: 'manage_accounts', permission: 'users.manage' },
          { id: 'contacts', to: '/settings/contacts', label: 'Contacts & escalation', icon: 'contacts', permission: 'contacts.write' },
          { id: 'locations', to: '/settings/locations', label: 'Locations', icon: 'apartment', permission: 'locations.write' },
          { id: 'templates', to: '/settings/templates', label: 'Templates', icon: 'widgets', permission: 'devices.write', keywords: 'service templates' },
          { id: 'discovery', to: '/settings/discovery', label: 'Discovery & import', icon: 'travel_explore', permission: 'discovery.read', keywords: 'scan bulk csv' },
          { id: 'status-page', to: '/settings/status-page', label: 'Status page', icon: 'public', permission: 'status_page.manage' },
          { id: 'backup', to: '/settings/backup', label: 'Backup', icon: 'backup', permission: 'settings.write', keywords: 'restore' },
          { id: 'system', to: '/settings/system', label: 'System', icon: 'info', keywords: 'version about host' },
        ],
      },
    ],
  },
];

/** Sub-tabs of Settings › Discovery & import. */
export const DISCOVERY_SECTIONS: NavSection[] = [
  { id: 'discovery', to: '/settings/discovery', label: 'Discovery', icon: 'travel_explore', permission: 'discovery.read', end: true },
  { id: 'import', to: '/settings/discovery/import', label: 'Bulk import', icon: 'upload_file', permission: 'devices.write' },
];

/**
 * v5.0 URLs and where they live now. Kept so bookmarks keep working;
 * `:id` is carried over and the query string is preserved.
 */
export const LEGACY_REDIRECTS: Array<{ from: string; to: string }> = [
  { from: '/incidents', to: '/alerts/incidents' },
  { from: '/incidents/:id', to: '/alerts/incidents/:id' },
  { from: '/maintenance', to: '/alerts/maintenance' },
  { from: '/devices/topology', to: '/topology' },
  { from: '/campus', to: '/topology/campus' },
  { from: '/inventory/cameras', to: '/devices?view=cameras' },
  { from: '/inventory/biometrics', to: '/devices?view=biometrics' },
  { from: '/sensors', to: '/devices/sensors' },
  { from: '/capture', to: '/flows/capture' },
  { from: '/reports/builder', to: '/reports/scheduled' },
  { from: '/discovery', to: '/settings/discovery' },
  { from: '/import', to: '/settings/discovery/import' },
  { from: '/service-templates', to: '/settings/templates' },
];

// ── Permissions ──────────────────────────────────────────────────────────

export function hasPermission(user: SessionUser | null | undefined, permission?: string): boolean {
  if (!permission) return true;
  if (user?.role === 'super_admin' || user?.role === 'admin') return true;
  return Boolean(user?.permissions?.includes(permission));
}

export function visibleSections(sections: NavSection[] | undefined, user: SessionUser | null | undefined): NavSection[] {
  return (sections ?? []).filter((s) => hasPermission(user, s.permission));
}

function itemVisible(item: NavItem, user: SessionUser | null | undefined): boolean {
  if (item.sections) return visibleSections(item.sections, user).length > 0;
  return hasPermission(user, item.permission);
}

/** Where an item's link goes: its first permitted section, else `to`. */
export function itemTarget(item: NavItem, user: SessionUser | null | undefined): string {
  return visibleSections(item.sections, user)[0]?.to ?? item.to;
}

export function visibleGroups(user: SessionUser | null | undefined): NavGroup[] {
  return NAV_GROUPS.map((g) => ({ ...g, items: g.items.filter((i) => itemVisible(i, user)) })).filter((g) => g.items.length > 0);
}

// ── Matching ─────────────────────────────────────────────────────────────

function pathMatches(pathname: string, to: string, end?: boolean): boolean {
  const base = to.split('?')[0]!;
  if (pathname === base) return true;
  if (end) return false;
  return base === '/' ? false : pathname.startsWith(`${base}/`);
}

/** Specificity of a match (longer prefix wins); -1 when it doesn't match. */
function matchScore(pathname: string, to: string, end?: boolean): number {
  return pathMatches(pathname, to, end) ? to.length : -1;
}

function itemScore(pathname: string, item: NavItem): number {
  const candidates = [
    matchScore(pathname, item.to, item.end),
    ...(item.sections ?? []).map((s) => matchScore(pathname, s.to, s.end)),
    ...(item.subpages ?? []).map((s) => matchScore(pathname, s.to)),
  ];
  return Math.max(...candidates);
}

export interface ActiveNav {
  group: NavGroup;
  item: NavItem;
  section?: NavSection;
}

/** The group/item/section a pathname belongs to. */
export function findActive(pathname: string, groups: NavGroup[] = NAV_GROUPS): ActiveNav | null {
  let best: ActiveNav | null = null;
  let bestScore = -1;
  for (const group of groups) {
    for (const item of group.items) {
      const score = itemScore(pathname, item);
      if (score > bestScore) {
        bestScore = score;
        best = { group, item };
      }
    }
  }
  if (!best) return null;
  const section = activeSection(pathname, best.item.sections ?? []);
  return section ? { ...best, section } : best;
}

/** Most specific section whose path matches. */
export function activeSection(pathname: string, sections: NavSection[]): NavSection | undefined {
  let best: NavSection | undefined;
  let bestScore = -1;
  for (const s of sections) {
    const score = matchScore(pathname, s.to, s.end);
    if (score > bestScore) {
      bestScore = score;
      best = s;
    }
  }
  return best;
}

export function isItemActive(pathname: string, item: NavItem): boolean {
  return itemScore(pathname, item) >= 0 && findActive(pathname)?.item.id === item.id;
}

// ── Breadcrumbs ──────────────────────────────────────────────────────────

export interface Crumb {
  label: string;
  to?: string;
}

/**
 * Group › Item › Section › Subpage for a pathname. The group has no link
 * (it is a heading, not a page). Duplicates collapse, so an item whose
 * target is its first section doesn't appear twice.
 */
export function breadcrumbsFor(pathname: string, user: SessionUser | null | undefined): Crumb[] {
  const active = findActive(pathname);
  if (!active) return [];
  const { group, item, section } = active;
  const crumbs: Crumb[] = [{ label: group.label }, { label: item.label, to: itemTarget(item, user) }];
  // Settings shows its sections in a side list, but they still read as a trail.
  if (section && (item.sections?.length ?? 0) > 1) crumbs.push({ label: section.label, to: section.to });
  if (item.id === 'settings' && pathname.startsWith('/settings/discovery/')) {
    const sub = activeSection(pathname, DISCOVERY_SECTIONS);
    if (sub && sub.id !== 'discovery') crumbs.push({ label: sub.label, to: sub.to });
  }
  const subpage = item.subpages?.find((s) => pathMatches(pathname, s.to));
  if (subpage) crumbs.push({ label: subpage.label, to: subpage.to });
  // Collapse consecutive crumbs that point at the same page.
  return crumbs.filter((c, i) => i === 0 || !c.to || c.to !== crumbs[i - 1]?.to);
}
