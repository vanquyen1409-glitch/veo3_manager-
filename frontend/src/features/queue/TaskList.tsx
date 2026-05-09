import { memo, useMemo } from 'react';
import { Trash2, ListVideo } from 'lucide-react';
import { useQueueStore, type TaskRow } from '../../store/queueStore';
import { truncate, formatRelative } from '../../lib/format';
import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import { cn } from '../../lib/cn';
import { taskPhaseLabel, taskPhaseTone } from '../../lib/taskStatus';

const Row = memo(function Row({ t, onDelete }: { t: TaskRow; onDelete: (id: string) => void }) {
  const phase = t.phase ?? (t.status as string) ?? 'pending';
  const total = t.config?.outputCount ?? 1;
  const done = t.videoPaths?.length ?? 0;
  const cur = t.progress?.i ?? done;
  const n = t.progress?.n ?? total;
  const pct = Math.min(100, Math.round((cur / Math.max(1, n)) * 100));

  return (
    <div className="rounded-md surface-panel px-3 py-2 text-xs">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <div className="text-strong truncate font-medium">{truncate(t.prompt, 80)}</div>
          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px] text-subtle">
            <span className={cn(taskPhaseTone(phase))}>{taskPhaseLabel(phase)}</span>
            <span>·</span>
            <span>{cur}/{n}</span>
            <span>·</span>
            <span>{formatRelative(t.createdAt as any)}</span>
          </div>
          {t.error && (
            <div className="mt-0.5 truncate text-[11px] text-red-600 dark:text-red-400">{truncate(t.error, 80)}</div>
          )}
        </div>
        {phase === 'pending' && (
          <button
            onClick={() => onDelete(t.id)}
            className="text-subtle hover-surface rounded p-1 hover:text-red-500"
            aria-label="Xóa"
          >
            <Trash2 size={14} />
          </button>
        )}
      </div>
      <div className="mt-2 h-1 w-full overflow-hidden rounded bg-slate-200 dark:bg-slate-800">
        <div
          className={cn(
            'h-full transition-all',
            phase === 'failed' || phase === 'cancelled' ? 'bg-red-500'
              : phase === 'succeeded' ? 'bg-emerald-500'
              : 'bg-accent-500',
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
});

export default function TaskList() {
  const tasks = useQueueStore((s) => s.tasks);
  const remove = useQueueStore((s) => s.remove);

  const all = useMemo(() => {
    const sorted = Object.values(tasks).sort((a, b) => {
      const ax = String(a.createdAt ?? '');
      const bx = String(b.createdAt ?? '');
      return ax.localeCompare(bx);
    });
    return sorted;
  }, [tasks]);
  const running = useMemo(() => all.filter((t) => t.status === 'running'), [all]);
  const pending = useMemo(() => all.filter((t) => t.status === 'pending'), [all]);

  return (
    <Card className="flex h-full flex-col">
      <CardHeader>
        <CardTitle>Hàng đợi ({all.length})</CardTitle>
        {(running.length > 0 || pending.length > 0) && (
          <span className="text-xs text-muted">
            {running.length} đang chạy · {pending.length} chờ
          </span>
        )}
      </CardHeader>

      <div className="flex flex-1 flex-col gap-2 overflow-auto">
        {all.length === 0 && (
          <div className="flex flex-1 flex-col items-center justify-center gap-2 py-8 text-center">
            <ListVideo size={32} className="text-slate-300 dark:text-slate-700" />
            <span className="text-xs text-subtle">Chưa có task nào trong hàng đợi</span>
          </div>
        )}
        {running.length > 0 && (
          <>
            <div className="label">Đang chạy</div>
            {running.map((t) => <Row key={t.id} t={t} onDelete={remove} />)}
          </>
        )}
        {pending.length > 0 && (
          <>
            <div className="label">Chờ xử lý</div>
            {pending.map((t) => <Row key={t.id} t={t} onDelete={remove} />)}
          </>
        )}
      </div>
    </Card>
  );
}
