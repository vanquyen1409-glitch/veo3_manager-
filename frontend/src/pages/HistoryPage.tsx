import { useEffect, useMemo, useState } from 'react';
import { Video as VideoIcon } from 'lucide-react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import HistoryFilters, { type StatusFilter } from '../features/history/HistoryFilters';
import HistoryDetailDrawer from '../features/history/HistoryDetailDrawer';
import { ListTasks } from '../../wailsjs/go/main/App';
import type { db } from '../../wailsjs/go/models';
import { formatDate, truncate } from '../lib/format';
import { cn } from '../lib/cn';
import { taskPhaseLabel, taskStatusBadge } from '../lib/taskStatus';
import { SkeletonRow } from '../components/ui/Skeleton';

const PAGE_SIZE = 50;

const COLS = 'grid-cols-[160px_1fr_120px_100px_70px]';

export default function HistoryPage() {
  const [status, setStatus] = useState<StatusFilter>('all');
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rows, setRows] = useState<db.Task[]>([]);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<db.Task | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(id);
  }, [search]);

  const filter = useMemo<db.ListFilter>(() => {
    const statuses = status === 'all'
      ? ['succeeded', 'failed', 'cancelled']
      : [status];
    return {
      statuses,
      search: debouncedSearch,
      offset: page * PAGE_SIZE,
      limit: PAGE_SIZE,
    } as unknown as db.ListFilter;
  }, [status, debouncedSearch, page, tick]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    ListTasks(filter)
      .then((r) => { if (!cancelled) setRows(r as db.Task[]); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [filter]);

  const showEmpty = !loading && rows.length === 0;

  return (
    <div className="relative flex h-full flex-col gap-4 p-6">
      <h1>Lịch sử</h1>

      <HistoryFilters
        status={status}
        search={search}
        onStatus={(s) => { setStatus(s); setPage(0); }}
        onSearch={(s) => { setSearch(s); setPage(0); }}
      />

      {showEmpty ? (
        <Card className="flex flex-1 flex-col items-center justify-center gap-3 py-12">
          <VideoIcon size={40} className="text-slate-300 dark:text-slate-700" />
          <span className="text-sm text-subtle">Chưa có lịch sử</span>
        </Card>
      ) : (
        <Card className="flex-1 overflow-hidden p-0">
          <div className="overflow-x-auto">
            <div className={cn('surface-elev grid border-b divider px-4 py-2 min-w-[700px]', COLS)}>
              <div className="label">Tạo lúc</div>
              <div className="label">Prompt</div>
              <div className="label">Trạng thái</div>
              <div className="label">Output</div>
              <div className="label">Mở</div>
            </div>
            <div className="divide-y divide-slate-200 dark:divide-slate-800 min-w-[700px]">
              {loading && rows.length === 0 && (
                <div className="flex flex-col">
                  {Array.from({ length: 6 }, (_, i) => <SkeletonRow key={i} />)}
                </div>
              )}
              {rows.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => setSelected(t)}
                  className={cn(
                    'grid w-full items-center px-4 py-2 text-left text-xs',
                    'transition-colors hover-surface active-surface',
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent-500',
                    COLS,
                  )}
                  aria-label={`Mở chi tiết task ${t.prompt.slice(0, 60)}`}
                >
                  <div className="text-muted">{formatDate(t.createdAt as any)}</div>
                  <div className="text-strong truncate">{truncate(t.prompt, 100)}</div>
                  <div>
                    <span className={cn(
                      'inline-flex rounded border px-1.5 py-0.5 text-[10px]',
                      taskStatusBadge(t.status),
                    )}>
                      {taskPhaseLabel(t.status)}
                    </span>
                  </div>
                  <div className="text-muted">{t.videoPaths?.length ?? 0} video</div>
                  <div className="text-subtle" aria-hidden="true">→</div>
                </button>
              ))}
            </div>
          </div>
        </Card>
      )}

      {!showEmpty && (
        <div className="flex items-center justify-between text-xs text-muted">
          <span>Trang {page + 1}</span>
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              Trước
            </Button>
            <Button variant="secondary" size="sm" disabled={rows.length < PAGE_SIZE} onClick={() => setPage((p) => p + 1)}>
              Sau
            </Button>
          </div>
        </div>
      )}

      {selected && (
        <HistoryDetailDrawer
          task={selected}
          onClose={() => setSelected(null)}
          onChanged={() => setTick((x) => x + 1)}
        />
      )}
    </div>
  );
}
