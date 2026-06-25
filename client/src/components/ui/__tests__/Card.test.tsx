import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Card from '../Card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>Content here</Card>);
    expect(screen.getByText('Content here')).toBeInTheDocument();
  });

  it('applies high variant class by default', () => {
    const { container } = render(<Card>High</Card>);
    expect(container.firstChild).toHaveClass('bg-surface-container-high');
  });

  it('applies low variant class', () => {
    const { container } = render(<Card variant="low">Low</Card>);
    expect(container.firstChild).toHaveClass('bg-surface-container-low');
  });

  it('applies highest variant class', () => {
    const { container } = render(<Card variant="highest">Highest</Card>);
    expect(container.firstChild).toHaveClass('bg-surface-container-highest');
  });

  it('applies border by default', () => {
    const { container } = render(<Card>Bordered</Card>);
    expect(container.firstChild).toHaveClass('border');
    expect(container.firstChild).toHaveClass('border-outline-variant/20');
  });

  it('hides border when borderless', () => {
    const { container } = render(<Card borderless>Borderless</Card>);
    expect(container.firstChild).not.toHaveClass('border');
  });

  it('applies hover class when hover is true', () => {
    const { container } = render(<Card hover>Hoverable</Card>);
    expect(container.firstChild).toHaveClass('hover:border-primary/50');
    expect(container.firstChild).toHaveClass('cursor-pointer');
  });

  it('applies custom className', () => {
    const { container } = render(<Card className="my-custom">Custom</Card>);
    expect(container.firstChild).toHaveClass('my-custom');
  });

  it('applies rounded-lg', () => {
    const { container } = render(<Card>Rounded</Card>);
    expect(container.firstChild).toHaveClass('rounded-lg');
  });
});
