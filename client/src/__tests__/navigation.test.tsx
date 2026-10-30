import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LEGACY_REDIRECTS, NAV_GROUPS, breadcrumbsFor, findActive, itemTarget, visibleGroups } from '../navigation';
import { LegacyRedirect } from '../components/layout/SectionHub';
import Layout, { NAV_RAIL_STORAGE_KEY } from '../components/Layout';
import PageHeader from '../components/ui/PageHeader';
import { sessionStore } from '../store/session';

const admin = { id: 1, username: 'admin', role: 'admin' };
const viewer = { id: 2, username: 'viewer', role: 'viewer', permissions: ['devices.read', 'alerts.read', 'reports.read'] };

describe('navigation model', () => {
  it('has about a dozen items in four groups', () => {
    expect(NAV_GROUPS.map((g) => g.label)).toEqual(['Monitor', 'Inventory', 'Analyze', 'Configure']);
    const count = NAV_GROUPS.flatMap((g) => g.items).length;
    expect(count).toBeLessThanOrEqual(12);
  });

  it('filters items and sections by permission', () => {
    const groups = visibleGroups(viewer);
    const ids = groups.flatMap((g) => g.items.map((i) => i.id));
    expect(ids).toContain('devices');
    expect(ids).not.toContain('logs');
    expect(ids).not.toContain('flows');
    // Reports: only Library + Builder are allowed, so the item links to Library.
    const reports = groups.flatMap((g) => g.items).find((i) => i.id === 'reports')!;
    expect(itemTarget(reports, viewer)).toBe('/reports/library');
  });

  it('resolves the active item and section for nested routes', () => {
    expect(findActive('/')?.item.id).toBe('overview');
    expect(findActive('/devices/42')?.item.id).toBe('devices');
    const incident = findActive('/alerts/incidents/7');
    expect(incident?.item.id).toBe('alerts');
    expect(incident?.section?.id).toBe('incidents');
    expect(findActive('/alerts')?.section?.id).toBe('alerts');
    expect(findActive('/reports')?.section?.id).toBe('builder');
    expect(findActive('/settings/discovery/import')?.section?.id).toBe('discovery');
    expect(findActive('/nope')).toBeNull();
  });

  it('builds breadcrumb trails without duplicate crumbs', () => {
    expect(breadcrumbsFor('/alerts', admin).map((c) => c.label)).toEqual(['Monitor', 'Alerts & incidents']);
    expect(breadcrumbsFor('/alerts/incidents', admin).map((c) => c.label)).toEqual(['Monitor', 'Alerts & incidents', 'Incidents']);
    expect(breadcrumbsFor('/settings/discovery/import', admin).map((c) => c.label)).toEqual(['Configure', 'Settings', 'Discovery & import', 'Bulk import']);
    expect(breadcrumbsFor('/devices/sensors', admin).map((c) => c.label)).toEqual(['Inventory', 'Devices', 'Sensor overview']);
  });
});

function Where() {
  const location = useLocation();
  return <output data-testid="where">{location.pathname + location.search}</output>;
}

function renderRedirect(url: string) {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        {LEGACY_REDIRECTS.map(({ from, to }) => <Route key={from} path={from} element={<LegacyRedirect to={to} />} />)}
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('legacy redirects', () => {
  it.each([
    ['/incidents/12', '/alerts/incidents/12'],
    ['/maintenance', '/alerts/maintenance'],
    ['/devices/topology', '/topology'],
    ['/campus?view=floor', '/topology/floor-plans'],
    ['/campus?view=map', '/topology/campus?view=map'],
    ['/inventory/cameras', '/devices?view=cameras'],
    ['/capture', '/flows/capture'],
    ['/reports/builder?tab=generated', '/reports/library'],
    ['/reports/builder', '/reports/scheduled'],
    ['/import', '/settings/discovery/import'],
    ['/service-templates', '/settings/templates'],
  ])('%s → %s', (from, to) => {
    renderRedirect(from);
    expect(screen.getByTestId('where')).toHaveTextContent(to);
  });
});

describe('Layout', () => {
  beforeEach(() => {
    sessionStore.setAuthenticated(admin);
    vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} }));
    Object.defineProperty(window, 'innerWidth', { value: 1440, configurable: true });
    localStorage.clear();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    sessionStore.setUnauthenticated();
  });

  const renderLayout = (url: string) =>
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false, enabled: false } } })}>
        <MemoryRouter initialEntries={[url]}>
          <Layout><PageHeader title="Incidents" /></Layout>
        </MemoryRouter>
      </QueryClientProvider>,
    );

  it('marks the hub item current on a nested route and shows breadcrumbs', () => {
    renderLayout('/alerts/incidents');
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('link', { name: 'Alerts & incidents' })).toHaveAttribute('aria-current', 'page');
    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' });
    expect(within(crumbs).getByRole('link', { name: 'Alerts & incidents' })).toHaveAttribute('href', '/alerts');
    expect(within(crumbs).getByText('Incidents')).toHaveAttribute('aria-current', 'page');
  });

  it('collapses to an icon rail and remembers it', async () => {
    const { unmount } = renderLayout('/');
    const toggle = screen.getByRole('button', { name: 'Collapse navigation' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await userEvent.click(toggle);
    expect(document.getElementById('app-sidebar')).toHaveAttribute('data-rail', 'true');
    expect(localStorage.getItem(NAV_RAIL_STORAGE_KEY)).toBe('1');
    // Labels stay available to assistive tech in the rail.
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getByRole('link', { name: 'Overview' })).toBeInTheDocument();
    unmount();

    renderLayout('/');
    expect(screen.getByRole('button', { name: 'Expand navigation' })).toHaveAttribute('aria-expanded', 'false');
  });
});
