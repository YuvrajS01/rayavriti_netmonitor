import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusDot, StatusPill } from '../Status';
import { healthMeta, severityMeta } from '../../../utils/status';

describe('StatusPill', () => {
  it('pairs a shape with a text label', () => {
    const { container } = render(<StatusPill status="down" />);
    expect(screen.getByText('Down')).toBeInTheDocument();
    expect(container.querySelector('svg rect')).not.toBeNull();
    expect(container.firstChild).toHaveClass('text-status-down', 'bg-status-down/12');
  });

  it('normalizes backend values', () => {
    render(<StatusPill status="degraded" />);
    expect(screen.getByText('Warning')).toBeInTheDocument();
  });

  it('accepts severity and health metas and custom text', () => {
    render(
      <>
        <StatusPill status={severityMeta('critical')} />
        <StatusPill status={healthMeta(45)} />
        <StatusPill status="up">SLA met</StatusPill>
      </>,
    );
    expect(screen.getByText('Critical')).toBeInTheDocument();
    expect(screen.getByText('Risk')).toBeInTheDocument();
    expect(screen.getByText('SLA met')).toBeInTheDocument();
  });
});

describe('StatusDot', () => {
  it('announces the status by default', () => {
    render(<StatusDot status="unknown" />);
    expect(screen.getByRole('img', { name: 'Unknown' })).toBeInTheDocument();
  });

  it('is decorative when label is empty', () => {
    const { container } = render(<StatusDot status="up" label="" />);
    expect(screen.queryByRole('img')).toBeNull();
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});
