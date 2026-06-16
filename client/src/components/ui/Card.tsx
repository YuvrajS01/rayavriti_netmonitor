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
      className={[
        bgMap[variant],
        'rounded-xl',
        borderless ? '' : 'border border-outline-variant/20',
        hover ? 'hover:border-primary/50 hover:shadow-[0_0_15px_rgba(217,253,58,0.1)] transition-[border-color,box-shadow] cursor-pointer' : '',
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      {children}
    </div>
  );
}
