import type { ReactNode, HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  /** `high`/`low` are the standard card (surface-1); `highest` is a raised inset (surface-2). */
  variant?: 'high' | 'low' | 'highest';
  /**
   * Only for cards that are themselves clickable: adds the hover border and
   * pointer. Static tiles must not look interactive.
   */
  hover?: boolean;
  borderless?: boolean;
}

const BG = {
  high: 'bg-surface-1',
  low: 'bg-surface-1',
  highest: 'bg-surface-2',
} as const;

export default function Card({ children, variant = 'high', hover = false, borderless = false, className = '', ...props }: CardProps) {
  return (
    <div
      className={[
        BG[variant],
        'rounded-lg',
        borderless ? '' : 'border border-line',
        hover ? 'hover:border-line-strong hover:bg-surface-2 transition-colors cursor-pointer' : '',
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      {children}
    </div>
  );
}
