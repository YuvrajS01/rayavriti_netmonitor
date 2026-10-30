import type { ReactNode, HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  variant?: 'high' | 'low' | 'highest';
  hover?: boolean;
  borderless?: boolean;
}

export default function Card({ children, variant = 'high', hover = false, borderless = false, className = '', ...props }: CardProps) {
  const bgMap = {
    high: 'bg-surface-1',
    low: 'bg-surface-1',
    highest: 'bg-surface-3',
  };

  return (
    <div
      role="region"
      className={[
        bgMap[variant],
        'rounded-lg',
        borderless ? '' : 'border border-line',
        hover ? 'hover:border-primary/50 transition-[border-color] cursor-pointer' : '',
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      {children}
    </div>
  );
}
