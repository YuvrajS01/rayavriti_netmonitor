import { useSyncExternalStore } from 'react';

/**
 * Theme and density preferences.
 *
 * The saved values live in localStorage (`nm-theme`, `nm-density`) and are
 * applied as `data-theme` / `data-density` on <html>. public/theme-init.js
 * applies them before first paint (the CSP forbids inline scripts); this
 * module re-applies them at boot, follows the OS while "system" is selected
 * and exposes React hooks for the controls.
 */

export type ThemePreference = 'system' | 'dark' | 'light';
export type ResolvedTheme = 'dark' | 'light';
export type Density = 'comfortable' | 'compact';

export const THEME_STORAGE_KEY = 'nm-theme';
export const DENSITY_STORAGE_KEY = 'nm-density';
export const THEME_CHANGE_EVENT = 'nm-theme-change';

const THEME_PREFERENCES: readonly ThemePreference[] = ['system', 'dark', 'light'];

function readStorage(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Storage blocked (private mode): the choice still applies for this tab.
  }
}

function systemPrefersLight(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-color-scheme: light)').matches;
}

export function getThemePreference(): ThemePreference {
  const saved = readStorage(THEME_STORAGE_KEY);
  return THEME_PREFERENCES.includes(saved as ThemePreference) ? (saved as ThemePreference) : 'system';
}

export function resolveTheme(pref: ThemePreference = getThemePreference()): ResolvedTheme {
  if (pref === 'system') return systemPrefersLight() ? 'light' : 'dark';
  return pref;
}

export function getDensity(): Density {
  return readStorage(DENSITY_STORAGE_KEY) === 'compact' ? 'compact' : 'comfortable';
}

const listeners = new Set<() => void>();
function notify() {
  listeners.forEach((listener) => listener());
  window.dispatchEvent(new Event(THEME_CHANGE_EVENT));
}

/** Writes data-theme/data-density on <html> from the saved preferences. */
export function applyPreferences() {
  const root = document.documentElement;
  root.dataset.theme = resolveTheme();
  if (getDensity() === 'compact') root.dataset.density = 'compact';
  else delete root.dataset.density;
}

export function setThemePreference(pref: ThemePreference) {
  writeStorage(THEME_STORAGE_KEY, pref);
  applyPreferences();
  notify();
}

export function setDensity(density: Density) {
  writeStorage(DENSITY_STORAGE_KEY, density);
  applyPreferences();
  notify();
}

let initialized = false;
/** Call once at boot (main.tsx), before the first render. */
export function initTheme() {
  applyPreferences();
  if (initialized || typeof window.matchMedia !== 'function') return;
  initialized = true;
  window.matchMedia('(prefers-color-scheme: light)').addEventListener('change', () => {
    if (getThemePreference() !== 'system') return;
    applyPreferences();
    notify();
  });
  // Another tab changed the preference.
  window.addEventListener('storage', (event) => {
    if (event.key !== THEME_STORAGE_KEY && event.key !== DENSITY_STORAGE_KEY) return;
    applyPreferences();
    notify();
  });
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useThemePreference(): ThemePreference {
  return useSyncExternalStore(subscribe, getThemePreference, () => 'system');
}

export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribe, () => resolveTheme(), () => 'dark');
}

export function useDensity(): Density {
  return useSyncExternalStore(subscribe, getDensity, () => 'comfortable');
}
