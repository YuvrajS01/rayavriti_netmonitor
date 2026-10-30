import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import CommandPalette from '../CommandPalette';
import { v1 } from '../../api/http';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

describe('CommandPalette', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 4, username: 'ops', role: 'admin' });
    localStorage.clear();
  });
  afterEach(() => {
    api.restore();
    sessionStore.setUnauthenticated();
  });

  it('is a combobox over a grouped listbox and jumps to settings sections', async () => {
    const onClose = vi.fn();
    renderPage(<CommandPalette open onClose={onClose} />);
    const input = screen.getByRole('combobox', { name: /Search pages/ });
    expect(input).toHaveFocus();
    await userEvent.type(input, 'backup');
    const listbox = screen.getByRole('listbox');
    const option = within(listbox).getByRole('option', { name: /Settings › Backup/ });
    expect(option).toHaveAttribute('aria-selected', 'true');
    expect(input).toHaveAttribute('aria-activedescendant', option.id);
    await userEvent.keyboard('{Enter}');
    expect(onClose).toHaveBeenCalled();
    expect(screen.getByTestId('location')).toHaveTextContent('/settings/backup');
    // Remembered as a recent item.
    expect(localStorage.getItem('nm-palette-recent:4')).toContain('Backup');
  });

  it('finds devices, locations and alerts through /search', async () => {
    api.onGet(/\/search\?q=core/).reply(200, { success: true, data: {
      devices: [{ id: 7, name: 'core-sw', ipAddress: '10.0.0.7', status: 'down', locationName: 'Library' }],
      locations: [{ id: 2, name: 'Core block', type: 'building' }],
      alerts: [{ id: 5, message: 'core-sw down', severity: 'critical', deviceName: 'core-sw', createdAt: '' }],
    } });
    renderPage(<CommandPalette open onClose={() => {}} initialQuery="core" />);
    const devices = await screen.findByRole('group', { name: 'Devices' });
    expect(within(devices).getByRole('option', { name: /core-sw/ })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Locations' })).toHaveTextContent('Core block');
    expect(screen.getByRole('group', { name: 'Alerts' })).toHaveTextContent('core-sw down');

    // Arrow down to the device (pages/actions matching "core" come first: none here).
    await userEvent.click(within(devices).getByRole('option', { name: /core-sw/ }));
    expect(screen.getByTestId('location')).toHaveTextContent('/devices/7');
  });

  it('acknowledges every active critical alert', async () => {
    api.onGet(/\/alerts\?status=active/).reply(200, { success: true, data: [
      { id: 1, deviceId: 1, severity: 'critical', message: 'a', status: 'active', createdAt: '' },
      { id: 2, deviceId: 1, severity: 'warning', message: 'b', status: 'active', createdAt: '' },
      { id: 3, deviceId: 1, severity: 'critical', message: 'c', status: 'active', createdAt: '' },
    ] });
    api.onPost('/alerts/bulk').reply(200, { success: true, data: { updated: [1, 3], skipped: [] } });
    renderPage(<CommandPalette open onClose={() => {}} />);
    await userEvent.type(screen.getByRole('combobox'), 'acknowledge all');
    await userEvent.keyboard('{Enter}');
    expect(await screen.findByText('Acknowledged 2 alerts')).toBeInTheDocument();
    expect(JSON.parse(api.history.post[0]!.data as string)).toEqual({ ids: [1, 3], action: 'acknowledge' });
  });
});
