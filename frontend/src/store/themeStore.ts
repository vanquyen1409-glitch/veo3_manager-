import { create } from 'zustand';

export type Theme = 'light' | 'dark';

const STORAGE_KEY = 'veo3-theme';

function readInitial(): Theme {
  if (typeof window === 'undefined') return 'dark';
  const saved = window.localStorage.getItem(STORAGE_KEY);
  if (saved === 'light' || saved === 'dark') return saved;
  // Honor system pref on first run, default to dark (matches the original UI).
  if (window.matchMedia?.('(prefers-color-scheme: light)').matches) return 'light';
  return 'dark';
}

function applyDocClass(theme: Theme) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.classList.toggle('dark', theme === 'dark');
  root.dataset.theme = theme;
}

interface ThemeState {
  theme: Theme;
  setTheme: (t: Theme) => void;
  toggleTheme: () => void;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: 'dark',
  setTheme: (t) => {
    window.localStorage.setItem(STORAGE_KEY, t);
    applyDocClass(t);
    set({ theme: t });
  },
  toggleTheme: () => {
    const next: Theme = get().theme === 'dark' ? 'light' : 'dark';
    get().setTheme(next);
  },
}));

/**
 * Boot the theme system once. Reads localStorage / system pref, applies the
 * `dark` class to <html>, and seeds the store. Call from main.tsx before
 * React mounts so first paint is correct.
 */
export function bootstrapTheme() {
  const initial = readInitial();
  applyDocClass(initial);
  useThemeStore.setState({ theme: initial });
}
