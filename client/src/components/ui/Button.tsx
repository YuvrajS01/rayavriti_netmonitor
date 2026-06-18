import type { ReactNode, ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-outline' | 'ghost' | 'primary-outline';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  children: ReactNode;
  icon?: string;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary/90',
  secondary: 'bg-surface-container-highest text-on-surface border border-outline-variant/30 hover:bg-surface-container-high',
  danger: 'bg-error text-on-error hover:bg-error/90',
  'danger-outline': 'border border-error text-error hover:bg-error/10',
  ghost: 'text-on-surface-variant hover:text-primary hover:bg-surface-container-high',
  'primary-outline': 'border border-primary/40 text-primary hover:bg-primary/5',
};

export default function Button({ variant = 'primary', children, icon, className = '', ...props }: ButtonProps) {
  return (
    <button
      className={[
        'font-headline font-bold text-sm uppercase tracking-wide rounded-md px-5 py-2.5 transition-[background-color,color,border-color]',
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
