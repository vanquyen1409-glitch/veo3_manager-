import { NavLink } from 'react-router-dom';
import { LayoutDashboard, ListVideo, History as HistoryIcon, Settings as SettingsIcon } from 'lucide-react';
import { cn } from '../lib/cn';
import BrowserStatusDot from './BrowserStatusDot';

const items = [
  { to: '/dashboard', label: 'Dashboard',  icon: LayoutDashboard },
  { to: '/queue',     label: 'Queue',      icon: ListVideo },
  { to: '/history',   label: 'Lịch sử',    icon: HistoryIcon },
  { to: '/settings',  label: 'Cài đặt',    icon: SettingsIcon },
];

export default function Sidebar() {
  return (
    <aside className="surface-app flex w-56 flex-none flex-col border-r divider">
      <nav className="flex flex-1 flex-col gap-1 p-3">
        {items.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md border px-3 py-2.5 text-sm transition',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500',
                isActive
                  ? 'accent-soft'
                  : 'text-muted border-transparent hover-surface hover:text-strong',
              )
            }
          >
            <Icon size={18} />
            <span className="font-medium">{label}</span>
          </NavLink>
        ))}
      </nav>
      <div className="border-t divider p-3">
        <BrowserStatusDot />
      </div>
    </aside>
  );
}
