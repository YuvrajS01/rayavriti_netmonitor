import type { ReactNode, ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-outline' | 'ghost' | 'primary-outline';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  children: ReactNode;
  icon?: string;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
  secondary: 'bg-surface-container-highest text-on-surface border border-outline-variant/30 hover:bg-surface-container-high focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
  danger: 'bg-error text-on-error hover:bg-error/90 focus-visible:ring-2 focus-visible:ring-error focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
  'danger-outline': 'border border-error text-error hover:bg-error/10 focus-visible:ring-2 focus-visible:ring-error focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
  ghost: 'text-on-surface-variant hover:text-primary hover:bg-surface-container-high focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
  'primary-outline': 'border border-primary/40 text-primary hover:bg-primary/5 focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:opacity-50 disabled:pointer-events-none',
};

export default function Button({ variant = 'primary', children, icon, className = '', ...props }: ButtonProps) {
  return (
    <button
      className={[
        'font-headline font-bold text-sm uppercase tracking-wide rounded-md px-5 py-2.5 min-h-11 min-w-11 transition-[background-color,color,border-color]',
        VARIANT_CLASSES[variant],
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      <span className="flex items-center justify-center gap-2">
        {icon && <span className="material-symbols-outlined text-lg">{icon}</span>}
        {children}
      </span>
    </button>
  );
}
