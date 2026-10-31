import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Dashboard from '../Dashboard';
import { v1 } from '../../api/http';
import type { Dashboard as SavedDashboard } from '../../api/types';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

const stats = { totalDevices: 42, onlineDevices: 40, offlineDevices: 2, warningDevices: 0, uptimePercent: 95, totalMetrics24h: 0, activeAlerts: 3, avgResponseTime: 0 };

const dashboard = (over: Partial<SavedDashboard>): SavedDashboard => ({
  id: 5,
  userId: 1,
  name: 'Core network',
  shared: false,
  isDefault: false,
  createdAt: '',
  updatedAt: '',
  layout: {
    version: 1,
    widgets: [
      { id: 'a', type: 'stat.total-devices', w: 3, h: 1 },
      { id: 'b', type: 'stat.active-alerts', w: 3, h: 1 },
    ],
  },
  ...over,
});

describe('Dashboards', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 1, username: 'ops', role: 'network_admin', permissions: ['devices.read', 'devices.write'] });
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
    try { sessionStorage.clear(); } catch { /* jsdom always has it */ }
    api.onGet('/system/stats').reply(200, { success: true, data: stats });
    api.onGet('/metrics/latest').reply(200, { success: true, data: [] });
  });
  afterEach(() => {
    api.restore();
    vi.unstubAllGlobals();
    sessionStore.setUnauthenticated();
  });

  it('opens the user’s default dashboard on /', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({ isDefault: true })] });
    renderPage(<Dashboard />, { url: '/', path: '/' });
    expect(await screen.findByRole('heading', { name: 'Core network' })).toBeInTheDocument();
    // The stat widget mounted (its number animates, so match the label).
    expect(await screen.findByText('Total devices')).toBeInTheDocument();
    expect(screen.getByText('Only you can see this dashboard.')).toBeInTheDocument();
  });

  it('shows another user’s shared dashboard read-only', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({ userId: 9, ownerName: 'Priya', shared: true })] });
    renderPage(<Dashboard />, { url: '/dashboards/5', path: '/dashboards/:id' });
    expect(await screen.findByRole('heading', { name: 'Core network' })).toBeInTheDocument();
    expect(screen.getByText(/Shared by Priya/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Duplicate to edit' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'More dashboard actions' }));
    expect(screen.queryByRole('button', { name: /Share with everyone|Delete/ })).not.toBeInTheDocument();
  });

  it('lets the owner reorder widgets and saves the new layout', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({})] });
    let body: { layout?: { widgets: Array<{ id: string }> } } = {};
    api.onPut('/dashboards/5').reply((config) => {
      body = JSON.parse(String(config.data)) as typeof body;
      return [200, { success: true, data: dashboard({ layout: body.layout as SavedDashboard['layout'] }) }];
    });
    renderPage(<Dashboard />, { url: '/dashboards/5', path: '/dashboards/:id' });

    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }));
    expect(screen.getByRole('heading', { name: 'Editing Core network' })).toBeInTheDocument();
    const save = screen.getByRole('button', { name: 'Save' });
    expect(save).toBeDisabled(); // nothing changed yet

    await userEvent.click(screen.getByRole('button', { name: 'Move Active alerts earlier' }));
    expect(screen.getByText('Active alerts moved to position 1 of 2.')).toBeInTheDocument();
    await userEvent.click(save);

    expect(await screen.findByText('Dashboard saved')).toBeInTheDocument();
    expect(body.layout?.widgets.map((w) => w.id)).toEqual(['b', 'a']);
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
  });

  it('adds a widget from the catalog and asks for its settings', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({ layout: { version: 1, widgets: [
      { id: 'a', type: 'stat.total-devices', w: 3, h: 1 },
      { id: 'b', type: 'metrics.latest', w: 6, h: 2 },
    ] } })] });
    api.onGet('/devices').reply(200, { success: true, data: [{ id: 3, name: 'edge-fw', ipAddress: '10.0.0.3', protocol: 'ping', port: 0, interval: 60, enabled: true, createdAt: '', updatedAt: '', status: 'up', tags: [] }] });
    renderPage(<Dashboard />, { url: '/dashboards/5', path: '/dashboards/:id' });

    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }));
    await userEvent.click(screen.getByRole('button', { name: 'Add widget' }));
    const catalog = await screen.findByRole('dialog', { name: 'Add a widget' });
    // Single-instance widgets already present can't be added twice; stat tiles can.
    expect(within(catalog).getByRole('button', { name: /Latest metrics/ })).toBeDisabled();
    expect(within(catalog).getByRole('button', { name: /Total devices/ })).toBeEnabled();
    await userEvent.click(within(catalog).getByRole('button', { name: /Device response history/ }));

    const settings = await screen.findByRole('dialog', { name: 'Device response history settings' });
    await userEvent.click(within(settings).getByRole('button', { name: 'Apply' }));
    expect(within(settings).getByRole('alert')).toHaveTextContent('Choose at least one device.');
    await userEvent.click(await within(settings).findByRole('checkbox', { name: /edge-fw/ }));
    await userEvent.click(within(settings).getByRole('button', { name: 'Apply' }));
    expect(screen.queryByRole('dialog', { name: /settings/ })).not.toBeInTheDocument();
    expect(screen.getByText(/3 of 48 widgets/)).toBeInTheDocument();
  });

  it('shows a placeholder for unknown widget types instead of failing', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({ layout: { version: 1, widgets: [{ id: 'x', type: 'future.widget', w: 6, h: 1 }] } })] });
    renderPage(<Dashboard />, { url: '/dashboards/5', path: '/dashboards/:id' });
    expect(await screen.findByText('Unsupported widget')).toBeInTheDocument();
    expect(screen.getByText('future.widget')).toBeInTheDocument();
  });

  it('hides sharing from users who can’t share', async () => {
    sessionStore.setAuthenticated({ id: 1, username: 'viewer', role: 'viewer', permissions: ['devices.read'] });
    api.onGet('/dashboards').reply(200, { success: true, data: [dashboard({})] });
    renderPage(<Dashboard />, { url: '/dashboards/5', path: '/dashboards/:id' });
    await userEvent.click(await screen.findByRole('button', { name: 'More dashboard actions' }));
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Share with everyone' })).not.toBeInTheDocument();
  });

  it('reports a dashboard that no longer exists', async () => {
    api.onGet('/dashboards').reply(200, { success: true, data: [] });
    renderPage(<Dashboard />, { url: '/dashboards/77', path: '/dashboards/:id' });
    expect(await screen.findByText('Dashboard not found')).toBeInTheDocument();
  });
});
