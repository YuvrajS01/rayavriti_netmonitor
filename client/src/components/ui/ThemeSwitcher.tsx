import { useId } from 'react';
import {
  setDensity,
  setThemePreference,
  useDensity,
  useThemePreference,
  type Density,
  type ThemePreference,
} from '../../theme/theme';

interface SegmentOption<T extends string> {
  value: T;
  label: string;
  icon: string;
}

interface SegmentedProps<T extends string> {
  legend: string;
  value: T;
  options: SegmentOption<T>[];
  onChange: (value: T) => void;
  size?: 'sm' | 'md';
  hideLegend?: boolean;
}

/** A radio group drawn as a segmented control (native radios, so arrow keys work). */
function Segmented<T extends string>({ legend, value, options, onChange, size = 'md', hideLegend = false }: SegmentedProps<T>) {
  const name = useId();
  return (
    <fieldset className="min-w-0">
      <legend className={hideLegend ? 'sr-only' : 'mb-2 text-xs font-medium text-fg-muted'}>{legend}</legend>
      <div className="inline-flex w-full rounded-md border border-line bg-inset p-0.5">
        {options.map((option) => {
          const checked = option.value === value;
          return (
            <label
              key={option.value}
              className={`relative flex flex-1 cursor-pointer items-center justify-center gap-1.5 rounded-[6px] font-medium transition-colors duration-[var(--duration-fast)] has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-focus ${
                size === 'sm' ? 'h-8 px-2 text-xs' : 'h-9 px-3 text-sm'
              } ${checked ? 'bg-surface-3 text-fg shadow-e1' : 'text-fg-muted hover:text-fg'}`}
            >
              <input
                type="radio"
                name={name}
                value={option.value}
                checked={checked}
                onChange={() => onChange(option.value)}
                className="sr-only"
              />
              <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{option.icon}</span>
              {option.label}
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

const THEME_OPTIONS: SegmentOption<ThemePreference>[] = [
  { value: 'system', label: 'System', icon: 'computer' },
  { value: 'dark', label: 'Dark', icon: 'dark_mode' },
  { value: 'light', label: 'Light', icon: 'light_mode' },
];

const DENSITY_OPTIONS: SegmentOption<Density>[] = [
  { value: 'comfortable', label: 'Comfortable', icon: 'density_medium' },
  { value: 'compact', label: 'Compact', icon: 'density_small' },
];

export function ThemeSwitcher({ size = 'md', hideLegend = false }: { size?: 'sm' | 'md'; hideLegend?: boolean }) {
  const pref = useThemePreference();
  return <Segmented legend="Theme" value={pref} options={THEME_OPTIONS} onChange={setThemePreference} size={size} hideLegend={hideLegend} />;
}

export function DensitySwitcher() {
  const density = useDensity();
  return <Segmented legend="Density" value={density} options={DENSITY_OPTIONS} onChange={setDensity} />;
}
