import { describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Dialog from '../Dialog';
import ConfirmDialog from '../../ConfirmDialog';

function Harness({ onClose }: { onClose?: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open settings</button>
      <Dialog
        open={open}
        onClose={() => { onClose?.(); setOpen(false); }}
        title="Edit device"
        description="Change polling settings"
        footer={<button onClick={() => setOpen(false)}>Save</button>}
      >
        <label>Name <input defaultValue="sw-a" /></label>
      </Dialog>
    </>
  );
}

describe('Dialog', () => {
  it('is labelled by its title and described by its description', async () => {
    render(<Harness />);
    await userEvent.click(screen.getByText('Open settings'));
    const dialog = screen.getByRole('dialog', { name: 'Edit device' });
    expect(dialog).toHaveAccessibleDescription('Change polling settings');
    expect(dialog).toHaveAttribute('open');
  });

  it('moves focus into the body and returns it to the opener on close', async () => {
    render(<Harness />);
    const opener = screen.getByText('Open settings');
    await userEvent.click(opener);
    expect(screen.getByLabelText('Name')).toHaveFocus();
    await userEvent.click(screen.getByText('Save'));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it('closes on Escape', async () => {
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    await userEvent.click(screen.getByText('Open settings'));
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('has a labelled close button', async () => {
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    await userEvent.click(screen.getByText('Open settings'));
    await userEvent.click(screen.getByRole('button', { name: 'Close dialog' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('renders nothing while closed', () => {
    render(<Dialog open={false} onClose={() => {}} title="Hidden">x</Dialog>);
    expect(screen.queryByText('Hidden')).toBeNull();
  });
});

describe('ConfirmDialog', () => {
  it('is an alertdialog that focuses Cancel, not the destructive action', () => {
    render(<ConfirmDialog open danger title="Delete device?" message="This cannot be undone." onConfirm={() => {}} onCancel={() => {}} />);
    expect(screen.getByRole('alertdialog', { name: 'Delete device?' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
  });

  it('uses a solid danger button for destructive confirms', () => {
    render(<ConfirmDialog open danger title="Delete?" message="m" confirmLabel="Delete" onConfirm={() => {}} onCancel={() => {}} />);
    const confirm = screen.getByRole('button', { name: 'Delete' });
    expect(confirm.className).toContain('bg-error');
    expect(confirm.className).toContain('text-on-error');
  });

  it('calls onConfirm / onCancel', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(<ConfirmDialog open title="Go?" message="m" confirmLabel="Go" onConfirm={onConfirm} onCancel={onCancel} />);
    await userEvent.click(screen.getByRole('button', { name: 'Go' }));
    expect(onConfirm).toHaveBeenCalled();
    await userEvent.keyboard('{Escape}');
    expect(onCancel).toHaveBeenCalled();
  });
});
