import type { ReactNode, ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-outline' | 'ghost' | 'primary-outline';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  children: ReactNode;
  icon?: string;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:brightness-110 shadow-[0_4px_20px_rgba(217,253,58,0.2)]',
  secondary: 'bg-surface-container-highest text-on-surface border border-outline-variant/30 hover:bg-surface-container-highest/80',
  danger: 'bg-error text-on-error hover:brightness-110',
  'danger-outline': 'border border-error text-error hover:bg-error/10',
  ghost: 'text-on-surface-variant hover:text-primary hover:bg-surface-container-high',
  'primary-outline': 'border border-primary/40 text-primary hover:bg-primary/5',
};

export default function Button({ variant = 'primary', children, icon, className = '', ...props }: ButtonProps) {
  return (
    <button
      className={[
        'font-headline font-bold text-xs uppercase tracking-widest rounded-lg active:scale-95 transition-[filter,transform,background-color,color]',
        VARIANT_CLASSES[variant],
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      <span className="flex items-center justify-center gap-2">
        {icon && <span className="material-symbols-outlined text-base">{icon}</span>}
        {children}
      </span>
    </button>
  );
}
