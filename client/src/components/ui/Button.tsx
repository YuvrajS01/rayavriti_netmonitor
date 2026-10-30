import { forwardRef, type ReactNode, type ButtonHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-outline' | 'ghost' | 'primary-outline';
type ButtonSize = 'sm' | 'md';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  children: ReactNode;
  icon?: string;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-on-primary hover:bg-primary-hover font-semibold',
  secondary: 'bg-surface-2 text-fg border border-line hover:bg-surface-3 hover:border-line-strong',
  // Solid: on-error on error clears 4.5:1 in both themes (see contrast.test.ts).
  danger: 'bg-error text-on-error hover:bg-error-hover font-semibold',
  'danger-outline': 'border border-error text-error hover:bg-error/12',
  ghost: 'text-fg-muted hover:text-fg hover:bg-surface-2',
  'primary-outline': 'border border-primary/40 text-primary hover:bg-primary/12',
};

const SIZE_CLASSES: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-sm gap-1.5',
  md: 'min-h-control px-4 text-sm gap-2',
};

const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', children, icon, className = '', ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      className={[
        'inline-flex items-center justify-center rounded-md font-medium transition-colors duration-[var(--duration-fast)] disabled:opacity-50 disabled:pointer-events-none',
        SIZE_CLASSES[size],
        VARIANT_CLASSES[variant],
        className,
      ].filter(Boolean).join(' ')}
      {...props}
    >
      {icon && <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{icon}</span>}
      {children}
    </button>
  );
});

export default Button;
