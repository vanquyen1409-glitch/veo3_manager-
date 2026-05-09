import { Moon, Sun } from 'lucide-react';
import { useThemeStore } from '../store/themeStore';
import { cn } from '../lib/cn';

interface Props {
  className?: string;
}

export default function ThemeToggle({ className }: Props) {
  const theme = useThemeStore((s) => s.theme);
  const toggle = useThemeStore((s) => s.toggleTheme);
  const isDark = theme === 'dark';

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={isDark ? 'Chuyển sang sáng' : 'Chuyển sang tối'}
      title={isDark ? 'Sáng' : 'Tối'}
      className={cn(
        'text-muted hover-surface hover:text-strong active-surface',
        'inline-flex h-7 w-7 items-center justify-center rounded-md transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500',
        className,
      )}
    >
      {isDark ? <Moon size={14} /> : <Sun size={14} />}
    </button>
  );
}
