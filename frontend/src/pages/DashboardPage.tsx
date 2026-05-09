import { useEffect, useState } from 'react';
import { Video, CheckCircle2, XCircle, TrendingUp, Clock, Film } from 'lucide-react';
import { GetStats } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import type { db } from '../../wailsjs/go/models';
import { cn } from '../lib/cn';
import { formatDuration } from '../lib/format';

interface Tile {
  label: string;
  value: string;
  icon: typeof Video;
  iconClass: string;
}

function StatTile({ label, value, icon: Icon, iconClass }: Tile) {
  return (
    <div className="rounded-xl surface-panel p-5">
      <div className="flex items-center gap-3">
        <span className={cn('flex h-9 w-9 items-center justify-center rounded-lg', iconClass)}>
          <Icon size={18} />
        </span>
        <span className="text-soft text-sm">{label}</span>
      </div>
      <div className="text-strong mt-4 text-3xl font-semibold">{value}</div>
    </div>
  );
}

export default function DashboardPage() {
  const [stats, setStats] = useState<db.Stats | null>(null);

  useEffect(() => {
    let cancelled = false;
    GetStats().then((s) => { if (!cancelled) setStats(s); }).catch(() => {});
    const off = EventsOn('queue:stats', (s: db.Stats) => setStats(s));
    return () => { cancelled = true; off?.(); };
  }, []);

  const total     = stats?.allTime.total     ?? 0;
  const succeeded = stats?.allTime.succeeded ?? 0;
  const failed    = stats?.allTime.failed    ?? 0;
  const rate      = total > 0 ? ((succeeded / total) * 100).toFixed(1) : '0.0';
  const avgMs     = stats?.avgDurationMs ?? 0;
  const totalVids = stats?.totalVideos   ?? 0;

  const tiles: Tile[] = [
    { label: 'Tổng tác vụ',     value: String(total),     icon: Video,        iconClass: 'bg-accent-500/15 text-accent-700 dark:text-accent-300'   },
    { label: 'Hoàn thành',      value: String(succeeded), icon: CheckCircle2, iconClass: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400' },
    { label: 'Thất bại',        value: String(failed),    icon: XCircle,      iconClass: 'bg-red-500/15 text-red-700 dark:text-red-400'         },
    { label: 'Tỷ lệ thành công', value: `${rate}%`,        icon: TrendingUp,   iconClass: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400' },
    { label: 'Thời gian TB',    value: avgMs > 0 ? formatDuration(avgMs) : '0s', icon: Clock, iconClass: 'bg-amber-500/15 text-amber-700 dark:text-amber-400' },
    { label: 'Tổng video',      value: String(totalVids), icon: Film,         iconClass: 'bg-accent-500/15 text-accent-700 dark:text-accent-300'   },
  ];

  return (
    <div className="flex flex-col gap-4 p-6">
      <h1>Dashboard</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {tiles.map((t) => (
          <StatTile key={t.label} {...t} />
        ))}
      </div>
    </div>
  );
}
