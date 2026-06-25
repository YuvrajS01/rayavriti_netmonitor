import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import SectionHeader from '../SectionHeader';

describe('SectionHeader', () => {
  it('renders title', () => {
    render(<SectionHeader title="Dashboard" />);
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  it('renders subtitle when provided', () => {
    render(<SectionHeader title="Dashboard" subtitle="Overview of all devices" />);
    expect(screen.getByText('Overview of all devices')).toBeInTheDocument();
  });

  it('does not render subtitle when omitted', () => {
    render(<SectionHeader title="Dashboard" />);
    expect(screen.queryByText('Overview')).not.toBeInTheDocument();
  });

  it('renders action when provided', () => {
    render(<SectionHeader title="Dashboard" action={<button>Add</button>} />);
    expect(screen.getByText('Add')).toBeInTheDocument();
  });

  it('does not render action when omitted', () => {
    const { container } = render(<SectionHeader title="Dashboard" />);
    const header = container.querySelector('header');
    expect(header!.children.length).toBe(1);
  });
});
