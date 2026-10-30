import { forwardRef, type ButtonHTMLAttributes } from 'react';

type IconButtonVariant = 'ghost' | 'secondary' | 'danger';
type IconButtonSize = 'sm' | 'md';

interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label' | 'children'> {
  /** Material Symbols ligature name. */
  icon: string;
  /** Required: the accessible name (rendered as aria-label and tooltip). */
  label: string;
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  /** For toggles (e.g. "pin", "mute"). */
  pressed?: boolean;
}

const VARIANT: Record<IconButtonVariant, string> = {
  ghost: 'text-fg-muted hover:bg-surface-2 hover:text-fg',
  secondary: 'border border-line bg-surface-2 text-fg hover:bg-surface-3',
  // Destructive row actions stay quiet until hovered: red is reserved for status.
  danger: 'text-fg-muted hover:bg-error/12 hover:text-error',
};

const SIZE: Record<IconButtonSize, { box: string; icon: string }> = {
  sm: { box: 'h-8 w-8', icon: 'text-[18px]' },
  md: { box: 'h-10 w-10', icon: 'text-[20px]' },
};

/** An icon-only button. `label` is mandatory so it is never announced as "button". */
const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { icon, label, variant = 'ghost', size = 'md', pressed, className = '', type = 'button', title, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      aria-pressed={pressed}
      title={title ?? label}
      className={`inline-flex shrink-0 items-center justify-center rounded-md transition-colors duration-[var(--duration-fast)] disabled:pointer-events-none disabled:opacity-50 ${SIZE[size].box} ${VARIANT[variant]} ${className}`}
      {...props}
    >
      <span className={`material-symbols-outlined ${SIZE[size].icon}`} aria-hidden="true">{icon}</span>
    </button>
  );
});

export default IconButton;
