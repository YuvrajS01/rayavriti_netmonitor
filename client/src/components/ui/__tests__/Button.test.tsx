import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Button from '../Button';

describe('Button', () => {
  it('renders text', () => {
    render(<Button>Submit</Button>);
    expect(screen.getByText('Submit')).toBeInTheDocument();
  });

  it('applies primary variant by default', () => {
    render(<Button>Primary</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('bg-primary');
    expect(btn.className).toContain('text-on-primary');
  });

  it('applies secondary variant', () => {
    render(<Button variant="secondary">Secondary</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('bg-surface-container-highest');
    expect(btn.className).toContain('border');
  });

  it('applies danger variant', () => {
    render(<Button variant="danger">Delete</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('bg-error');
    expect(btn.className).toContain('text-on-error');
  });

  it('applies danger-outline variant', () => {
    render(<Button variant="danger-outline">Danger Outline</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('border-error');
    expect(btn.className).toContain('text-error');
  });

  it('applies ghost variant', () => {
    render(<Button variant="ghost">Ghost</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('text-on-surface-variant');
  });

  it('applies primary-outline variant', () => {
    render(<Button variant="primary-outline">Outline</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('border-primary/40');
    expect(btn.className).toContain('text-primary');
  });

  it('renders icon when provided', () => {
    render(<Button icon="add">Add Item</Button>);
    expect(screen.getByText('add')).toBeInTheDocument();
  });

  it('does not render icon when omitted', () => {
    render(<Button>No Icon</Button>);
    expect(screen.queryByText('add')).not.toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Click Me</Button>);
    await userEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('is disabled when disabled prop is set', () => {
    render(<Button disabled>Disabled</Button>);
    expect(screen.getByRole('button')).toBeDisabled();
  });

  it('applies custom className', () => {
    render(<Button className="extra-class">Custom</Button>);
    expect(screen.getByRole('button').className).toContain('extra-class');
  });

  it('applies font-body, font-medium base classes', () => {
    render(<Button>Styled</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toContain('font-body');
    expect(btn.className).toContain('font-medium');
  });
});
