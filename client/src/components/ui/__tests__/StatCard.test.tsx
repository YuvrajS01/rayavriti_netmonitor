import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatCard from '../StatCard';

describe('StatCard', () => {
  it('renders label', () => {
    render(<StatCard label="Total Devices" value={42} />);
    expect(screen.getByText('Total Devices')).toBeInTheDocument();
  });

  it('renders numeric value', () => {
    render(<StatCard label="Uptime" value={99.9} />);
    expect(screen.getByText('99.9')).toBeInTheDocument();
  });

  it('renders string value', () => {
    render(<StatCard label="Status" value="All Clear" />);
    expect(screen.getByText('All Clear')).toBeInTheDocument();
  });

  it('applies default text-fg color', () => {
    render(<StatCard label="Test" value="Val" />);
    const valueEl = screen.getByText('Val');
    expect(valueEl.className).toContain('text-fg');
  });

  it('applies custom color', () => {
    render(<StatCard label="Test" value="Val" color="text-error" />);
    const valueEl = screen.getByText('Val');
    expect(valueEl.className).toContain('text-error');
  });

  it('renders icon when provided', () => {
    render(<StatCard label="Test" value="Val" icon="check_circle" />);
    expect(screen.getByText('check_circle')).toBeInTheDocument();
  });

  it('does not render icon when omitted', () => {
    render(<StatCard label="Test" value="Val" />);
    expect(screen.queryByText('check_circle')).not.toBeInTheDocument();
  });

  it('uses the surface-1 card background', () => {
    const { container } = render(<StatCard label="Test" value="Val" />);
    expect(container.firstChild).toHaveClass('bg-surface-1');
  });

  it('applies rounded-lg', () => {
    const { container } = render(<StatCard label="Test" value="Val" />);
    expect(container.firstChild).toHaveClass('rounded-lg');
  });

  it('renders the value large with tabular numbers', () => {
    render(<StatCard label="Test" value="Big" />);
    const valueEl = screen.getByText('Big');
    expect(valueEl.className).toContain('tabular-nums');
    expect(valueEl.className).toContain('text-2xl');
  });

  it('is a static tile (no hover lift)', () => {
    const { container } = render(<StatCard label="Test" value={3} />);
    expect((container.firstChild as HTMLElement).className).not.toMatch(/hover:|stat-card/);
  });
});
