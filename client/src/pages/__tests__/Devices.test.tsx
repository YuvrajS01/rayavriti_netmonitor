import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Devices from '../Devices';
import { v1 } from '../../api/http';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

const device = (id: number, name: string, extra: Record<string, unknown> = {}) => ({
  id, name, ipAddress: `10.0.0.${id}`, protocol: 'ping', port: 0, interval: 60, enabled: true, createdAt: '', updatedAt: '', status: 'up', tags: [], ...extra,
});

describe('Devices page', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 9, username: 'admin', role: 'admin' });
    localStorage.clear();
    api.onGet('/devices').reply(200, {
      success: true,
      data: [
        device(1, 'core-sw', { locationId: 5 }),
        device(2, 'gate-cam', { protocol: 'camera' }),
        device(3, 'edge-fw', { status: 'down' }),
      ],
    });
    api.onGet('/metrics/latest').reply(200, { success: true, data: [] });
    api.onGet('/locations').reply(200, { success: true, data: [{ id: 5, name: 'Library' }] });
  });
  afterEach(() => {
    api.restore();
    sessionStore.setUnauthenticated();
  });

  it('lists devices in a table, worst status first, with links to the detail page', async () => {
    renderPage(<Devices />, { url: '/devices' });
    const grid = await screen.findByRole('grid', { name: 'Devices' });
    const rows = within(grid).getAllByRole('row').filter((r) => r.hasAttribute('aria-selected'));
    expect(rows[0]).toHaveTextContent('edge-fw');
    expect(within(grid).getByRole('link', { name: 'core-sw' })).toHaveAttribute('href', '/devices/1');
    expect(await within(grid).findByText('Library')).toBeInTheDocument();
  });

  it('treats cameras as a saved view selected by ?view=', async () => {
    renderPage(<Devices />, { url: '/devices?view=cameras' });
    const grid = await screen.findByRole('grid', { name: 'Devices' });
    expect(within(grid).getByText('gate-cam')).toBeInTheDocument();
    expect(within(grid).queryByText('core-sw')).toBeNull();
    expect(screen.getByRole('combobox', { name: 'View' })).toHaveDisplayValue('Cameras');
  });

  it('bulk-deletes only after the count is typed', async () => {
    api.onDelete(/\/devices\/\d+/).reply(200, { success: true });
    renderPage(<Devices />, { url: '/devices' });
    const grid = await screen.findByRole('grid', { name: 'Devices' });
    await userEvent.click(within(grid).getByRole('checkbox', { name: 'Select core-sw' }));
    await userEvent.click(within(grid).getByRole('checkbox', { name: 'Select edge-fw' }));
    await userEvent.click(screen.getByRole('button', { name: 'Delete…' }));

    const dialog = screen.getByRole('alertdialog');
    const confirm = within(dialog).getByRole('button', { name: 'Delete devices' });
    expect(confirm).toBeDisabled();
    await userEvent.type(within(dialog).getByRole('textbox'), 'delete 2');
    expect(confirm).toBeEnabled();
    await userEvent.click(confirm);

    expect(await screen.findByText('Deleted 2 devices')).toBeInTheDocument();
    expect(api.history.delete.map((r) => r.url).sort()).toEqual(['/devices/1', '/devices/3']);
  });

  it('adopts the header search from ?search=', async () => {
    renderPage(<Devices />, { url: '/devices?search=edge' });
    const grid = await screen.findByRole('grid', { name: 'Devices' });
    expect(within(grid).getByText('edge-fw')).toBeInTheDocument();
    expect(within(grid).queryByText('core-sw')).toBeNull();
  });
});
