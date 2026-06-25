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

  it('applies default text-primary color', () => {
    render(<StatCard label="Test" value="Val" />);
    const valueEl = screen.getByText('Val');
    expect(valueEl.className).toContain('text-primary');
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

  it('applies surface-container-low bg', () => {
    const { container } = render(<StatCard label="Test" value="Val" />);
    expect(container.firstChild).toHaveClass('bg-surface-container-low');
  });

  it('applies rounded-lg', () => {
    const { container } = render(<StatCard label="Test" value="Val" />);
    expect(container.firstChild).toHaveClass('rounded-lg');
  });

  it('applies font-headline and text-2xl to value', () => {
    render(<StatCard label="Test" value="Big" />);
    const valueEl = screen.getByText('Big');
    expect(valueEl.className).toContain('font-headline');
    expect(valueEl.className).toContain('text-2xl');
  });
});
