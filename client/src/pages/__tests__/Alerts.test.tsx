import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Alerts from '../Alerts';
import { v1 } from '../../api/http';
import { applyAlertUpdated } from '../../api/liveCache';
import { queryKeys } from '../../api/queryKeys';
import type { Alert } from '../../api/types';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

const alert = (id: number, extra: Partial<Alert> = {}): Alert => ({
  id, deviceId: id * 10, deviceName: `dev-${id}`, severity: 'critical', message: `Alert ${id}`, status: 'active', createdAt: new Date(Date.now() - id * 60_000).toISOString(), ...extra,
});

const rowsOf = (grid: HTMLElement) => within(grid).getAllByRole('row').filter((r) => r.hasAttribute('aria-selected'));

describe('Alerts workbench', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 3, username: 'asha', role: 'admin' });
    localStorage.clear();
    api.onGet(/\/alerts\?status=active/).reply(200, { success: true, data: { alerts: [
      alert(1),
      alert(2, { severity: 'warning', assignedTo: 3, assignedToName: 'Asha' }),
      alert(3, { severity: 'info' }),
    ], total: 3 } });
    api.onGet('/alerts/counts').reply(200, { success: true, data: { active: 3, acknowledged: 0, resolved: 0 } });
    api.onGet('/devices').reply(200, { success: true, data: [] });
    api.onGet('/locations').reply(200, { success: true, data: [] });
  });
  afterEach(() => {
    api.restore();
    sessionStore.setUnauthenticated();
  });

  it('filters by severity and assignee=me from the URL', async () => {
    renderPage(<Alerts />, { url: '/alerts?assignee=me' });
    const grid = await screen.findByRole('grid', { name: 'Active alerts' });
    expect(rowsOf(grid)).toHaveLength(1);
    expect(rowsOf(grid)[0]).toHaveTextContent('Alert 2');
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Assignee' }), '');
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Severity' }), 'info');
    expect(rowsOf(grid)).toHaveLength(1);
    expect(rowsOf(grid)[0]).toHaveTextContent('Alert 3');
  });

  it('acknowledges optimistically and rolls back on failure', async () => {
    api.onPost('/alerts/1/acknowledge').reply(() => new Promise((resolve) => setTimeout(() => resolve([500]), 100)));
    renderPage(<Alerts />, { url: '/alerts' });
    const grid = await screen.findByRole('grid', { name: 'Active alerts' });
    await userEvent.click(within(grid).getByRole('button', { name: 'Acknowledge Alert 1' }));
    // Gone from the active list at once…
    await waitFor(() => expect(within(grid).queryByText('Alert 1')).toBeNull());
    // …and back after the server refuses.
    expect(await within(grid).findByText('Alert 1')).toBeInTheDocument();
    expect(await screen.findByText(/Could not acknowledge the alert/)).toBeInTheDocument();
  });

  it('triages with the keyboard: j/k move, x selects, a bulk-acknowledges', async () => {
    api.onPost('/alerts/bulk').reply((config) => {
      const body = JSON.parse(config.data as string) as { ids: number[]; action: string; note?: string };
      expect(body).toEqual({ ids: [1, 2], action: 'acknowledge', note: 'flapping uplink' });
      return [200, { success: true, data: { updated: [1, 2], skipped: [] } }];
    });
    renderPage(<Alerts />, { url: '/alerts' });
    const grid = await screen.findByRole('grid', { name: 'Active alerts' });

    fireEvent.keyDown(document.body, { key: 'j' });
    fireEvent.keyDown(document.body, { key: 'x' });
    fireEvent.keyDown(document.body, { key: 'j' });
    fireEvent.keyDown(document.body, { key: 'x' });
    expect(screen.getByRole('region', { name: 'Bulk actions' })).toHaveTextContent('2 selected');
    expect(grid.getAttribute('aria-activedescendant')).toMatch(/row-2$/);

    fireEvent.keyDown(document.body, { key: 'a' });
    const dialog = await screen.findByRole('dialog', { name: 'Acknowledge 2 alerts?' });
    await userEvent.type(within(dialog).getByLabelText(/Note/), 'flapping uplink');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Acknowledge' }));

    expect(await screen.findByText('Acknowledged 2 alerts')).toBeInTheDocument();
  });

  it('falls back to per-alert calls when the server has no bulk endpoint', async () => {
    api.onPost('/alerts/bulk').reply(404);
    api.onPost(/\/alerts\/\d+\/resolve/).reply(200, { success: true });
    renderPage(<Alerts />, { url: '/alerts' });
    const grid = await screen.findByRole('grid', { name: 'Active alerts' });
    await userEvent.click(within(grid).getByRole('checkbox', { name: /Select all 3 rows/ }));
    await userEvent.click(within(screen.getByRole('region', { name: 'Bulk actions' })).getByRole('button', { name: 'Resolve' }));
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Resolve' }));
    expect(await screen.findByText('Resolved 3 alerts')).toBeInTheDocument();
    expect(api.history.post.filter((r) => r.url?.endsWith('/resolve'))).toHaveLength(3);
  });

  it('applies alert:updated in place (assignee change from another operator)', async () => {
    const { client } = renderPage(<Alerts />, { url: '/alerts' });
    const grid = await screen.findByRole('grid', { name: 'Active alerts' });
    act(() => applyAlertUpdated(client, alert(3, { severity: 'info', assignedTo: 9, assignedToName: 'Ravi' })));
    expect(await within(grid).findByText('Ravi')).toBeInTheDocument();
    expect(client.getQueryData<{ data: Alert[] }>(queryKeys.alerts.list('active', 200))!.data).toHaveLength(3);
  });

  it('opens notes with n and posts a note', async () => {
    api.onGet('/alerts/1/notes').reply(200, { success: true, data: [{ id: 1, alertId: 1, text: 'Looking', author: 'ravi', createdAt: new Date().toISOString() }] });
    api.onPost('/alerts/1/notes').reply(200, { success: true, data: { id: 2, alertId: 1, text: 'Rebooted', author: 'asha', createdAt: new Date().toISOString() } });
    api.onGet('/users/assignable').reply(200, { success: true, data: [{ id: 3, username: 'asha', displayName: 'Asha' }] });
    renderPage(<Alerts />, { url: '/alerts' });
    await screen.findByRole('grid', { name: 'Active alerts' });
    fireEvent.keyDown(document.body, { key: 'j' });
    fireEvent.keyDown(document.body, { key: 'n' });
    const drawer = await screen.findByRole('dialog', { name: 'Alert 1' });
    expect(await within(drawer).findByText('Looking')).toBeInTheDocument();
    await userEvent.type(within(drawer).getByLabelText('Add a note'), 'Rebooted');
    await userEvent.click(within(drawer).getByRole('button', { name: 'Add note' }));
    expect(await within(drawer).findByText('Rebooted')).toBeInTheDocument();
  });
});
