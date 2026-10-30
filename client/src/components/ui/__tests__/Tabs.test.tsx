import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { TabPanel, Tabs } from '../Tabs';
import { useTabParam } from '../../../hooks/useTabParam';
import IconButton from '../IconButton';

const IDS = ['summary', 'devices', 'alerts'] as const;

function Location() {
  const location = useLocation();
  return <output data-testid="search">{location.search}</output>;
}

function Harness() {
  const [tab, setTab] = useTabParam(IDS);
  return (
    <>
      <Tabs
        label="Report sections"
        idBase="r"
        value={tab}
        onChange={setTab}
        items={[{ id: 'summary', label: 'Summary' }, { id: 'devices', label: 'Devices' }, { id: 'alerts', label: 'Alerts', badge: 3 }]}
      />
      {IDS.map((id) => <TabPanel key={id} idBase="r" id={id} value={tab}>{id} panel</TabPanel>)}
      <Location />
    </>
  );
}

function renderAt(url: string) {
  return render(<MemoryRouter initialEntries={[url]}><Harness /></MemoryRouter>);
}

describe('Tabs', () => {
  it('exposes tablist/tab/tabpanel with selection and linkage', () => {
    renderAt('/reports');
    expect(screen.getByRole('tablist', { name: 'Report sections' })).toBeInTheDocument();
    const summary = screen.getByRole('tab', { name: 'Summary' });
    expect(summary).toHaveAttribute('aria-selected', 'true');
    expect(summary).toHaveAttribute('tabindex', '0');
    expect(screen.getByRole('tab', { name: 'Devices' })).toHaveAttribute('tabindex', '-1');
    const panel = screen.getByRole('tabpanel', { name: 'Summary' });
    expect(summary).toHaveAttribute('aria-controls', panel.id);
  });

  it('reads the initial tab from ?tab= and ignores unknown values', () => {
    renderAt('/reports?tab=alerts');
    expect(screen.getByRole('tab', { name: /Alerts/ })).toHaveAttribute('aria-selected', 'true');
    renderAt('/reports?tab=nope');
    expect(screen.getAllByRole('tab', { name: 'Summary' }).at(-1)).toHaveAttribute('aria-selected', 'true');
  });

  it('arrow keys move focus and selection and sync the URL', async () => {
    renderAt('/reports');
    screen.getByRole('tab', { name: 'Summary' }).focus();
    await userEvent.keyboard('{ArrowRight}');
    const devices = screen.getByRole('tab', { name: 'Devices' });
    expect(devices).toHaveFocus();
    expect(devices).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('devices panel')).toBeInTheDocument();
    expect(screen.getByTestId('search')).toHaveTextContent('?tab=devices');
    await userEvent.keyboard('{End}');
    expect(screen.getByRole('tab', { name: /Alerts/ })).toHaveFocus();
    await userEvent.keyboard('{Home}');
    expect(screen.getByTestId('search')).toHaveTextContent(/^$/);
    await userEvent.keyboard('{ArrowLeft}');
    expect(screen.getByRole('tab', { name: /Alerts/ })).toHaveAttribute('aria-selected', 'true');
  });
});

describe('IconButton', () => {
  it('renders its required label as the accessible name and hides the glyph', () => {
    render(<IconButton icon="delete" label="Delete device" />);
    const button = screen.getByRole('button', { name: 'Delete device' });
    expect(button).toHaveAttribute('type', 'button');
    expect(button.querySelector('[aria-hidden="true"]')).toHaveTextContent('delete');
  });
});
