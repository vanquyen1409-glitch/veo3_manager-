import { useEffect, useRef } from 'react';
import { X, RotateCcw, Folder, Trash2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import VideoCarousel from './VideoCarousel';
import { formatDate, formatDuration } from '../../lib/format';
import {
  RequeueTask,
  DeleteTask,
  OpenInExplorer,
} from '../../../wailsjs/go/main/App';
import type { db } from '../../../wailsjs/go/models';
import { useAppStore } from '../../store/appStore';

interface Props {
  task: db.Task;
  onClose: () => void;
  onChanged: () => void;
}

export default function HistoryDetailDrawer({ task, onClose, onChanged }: Props) {
  const pushToast = useAppStore((s) => s.pushToast);
  const drawerRef = useRef<HTMLDivElement>(null);
  const closeBtnRef = useRef<HTMLButtonElement>(null);

  // Esc-to-close + initial focus on open. Keeps the previously-focused
  // element so we can restore focus when the drawer closes.
  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null;
    closeBtnRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Escape') return;
      e.preventDefault();
      onClose();
    }
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      previouslyFocused?.focus?.();
    };
  }, [onClose]);
  const elapsed =
    task.startedAt && task.finishedAt
      ? new Date(task.finishedAt as any).getTime() -
        new Date(task.startedAt as any).getTime()
      : 0;

  async function requeue() {
    try {
      await RequeueTask(task.id);
      pushToast({ kind: 'success', title: 'Đã thêm lại vào hàng đợi' });
      onChanged();
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
    }
  }

  const openConfirm = useAppStore((s) => s.openConfirm);

  function remove() {
    openConfirm({
      title: 'Xóa task khỏi lịch sử?',
      body: 'File video vẫn còn trên ổ đĩa, chỉ xóa bản ghi trong DB.',
      confirmLabel: 'Xóa',
      danger: true,
      onConfirm: async () => {
        try {
          await DeleteTask(task.id);
          pushToast({ kind: 'success', title: 'Đã xóa' });
          onClose();
          onChanged();
        } catch (e: any) {
          pushToast({ kind: 'error', title: 'Xóa thất bại', body: String(e?.message ?? e) });
        }
      },
    });
  }

  function openFolder() {
    const p = task.videoPaths?.[0];
    if (!p) return;
    OpenInExplorer(p).catch(() => {});
  }

  return (
    <div
      ref={drawerRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby="task-drawer-title"
      className="surface-app absolute inset-y-0 right-0 z-30 flex w-full max-w-[36rem] flex-col border-l divider shadow-2xl"
    >
      <header className="flex items-center justify-between border-b divider px-4 py-3">
        <div>
          <div className="label">Task</div>
          <div id="task-drawer-title" className="text-soft font-mono text-sm">{task.id.slice(0, 8)}</div>
        </div>
        <button
          ref={closeBtnRef}
          type="button"
          onClick={onClose}
          aria-label="Đóng"
          className="text-muted hover-surface hover:text-strong rounded p-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500"
        >
          <X size={16} />
        </button>
      </header>

      <div className="flex-1 overflow-auto p-4 text-sm">
        <div className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
          <span>tạo: {formatDate(task.createdAt as any)}</span>
          <span>kết thúc: {formatDate(task.finishedAt as any)}</span>
          <span>thời gian: {elapsed > 0 ? formatDuration(elapsed) : '—'}</span>
          <span>trạng thái: <b className="text-strong">{task.status}</b></span>
        </div>

        <div className="label mb-1">Prompt</div>
        <pre className="mb-4 whitespace-pre-wrap rounded-md surface-panel p-3 text-xs">
          {task.prompt}
        </pre>

        <div className="label mb-1">Cấu hình</div>
        <pre className="text-soft mb-4 whitespace-pre-wrap rounded-md surface-panel p-3 text-xs">
          {JSON.stringify(task.config, null, 2)}
        </pre>

        {task.error && (
          <>
            <div className="label mb-1">Lỗi</div>
            <pre className="mb-4 whitespace-pre-wrap rounded-md border border-red-300 bg-red-50 p-3 text-xs text-red-800 dark:border-red-700 dark:bg-slate-900 dark:text-red-300">
              {task.error}
            </pre>
          </>
        )}

        <div className="label mb-2">Video</div>
        <VideoCarousel paths={task.videoPaths ?? []} />
      </div>

      <footer className="flex flex-wrap gap-2 border-t divider p-3">
        <Button variant="secondary" onClick={requeue}>
          <RotateCcw size={14} /> Thêm lại
        </Button>
        {task.videoPaths && task.videoPaths.length > 0 && (
          <Button variant="ghost" onClick={openFolder}>
            <Folder size={14} /> Mở thư mục
          </Button>
        )}
        <div className="flex-1" />
        <Button variant="danger" onClick={remove}>
          <Trash2 size={14} /> Xóa
        </Button>
      </footer>
    </div>
  );
}
