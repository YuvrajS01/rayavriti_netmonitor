import type { ReactNode, ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-outline' | 'ghost' | 'primary-outline';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  children: ReactNode;
  icon?: string;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary-dim font-headline font-semibold',
  secondary: 'bg-surface-container-highest text-on-surface border border-outline-variant/30 hover:bg-surface-bright',
  danger: 'bg-error text-on-error hover:bg-error-dim',
  'danger-outline': 'border border-error text-error hover:bg-error-container',
  ghost: 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container',
  'primary-outline': 'border border-primary/40 text-primary hover:bg-primary-container',
};

export default function Button({ variant = 'primary', children, icon, className = '', ...props }: ButtonProps) {
  return (
    <button
      className={[
        'font-body font-medium text-sm rounded-md px-5 py-2.5 min-h-11 min-w-11 transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none',
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
