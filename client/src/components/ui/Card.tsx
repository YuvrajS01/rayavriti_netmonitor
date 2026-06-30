import type { ReactNode, HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  variant?: 'high' | 'low' | 'highest';
  hover?: boolean;
  borderless?: boolean;
}

export default function Card({ children, variant = 'high', hover = false, borderless = false, className = '', ...props }: CardProps) {
  const bgMap = {
    high: 'bg-surface-container-high',
    low: 'bg-surface-container-low',
    highest: 'bg-surface-container-highest',
  };

  return (
    <div
      role="region"
      className={[
        bgMap[variant],
        'rounded-lg',
        borderless ? '' : 'border border-outline-variant/20',
        hover ? 'hover:border-primary/50 transition-[border-color] cursor-pointer' : '',
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      {children}
    </div>
  );
}
