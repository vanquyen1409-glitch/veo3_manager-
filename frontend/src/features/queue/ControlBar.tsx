import { Loader2, Pause, Play, Square } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { useAppStore } from '../../store/appStore';
import { useQueueStore } from '../../store/queueStore';
import { StartQueue, PauseQueue, ResumeQueue, StopQueue } from '../../../wailsjs/go/main/App';

export default function ControlBar() {
  const queueState = useAppStore((s) => s.queueState);
  const pushToast = useAppStore((s) => s.pushToast);
  // Subscribe to a derived primitive — Zustand only re-renders when the
  // count changes, instead of on every task event (which mutates the map).
  const pendingCount = useQueueStore(
    (s) => Object.values(s.tasks).filter((t) => t.status === 'pending').length,
  );

  async function start() {
    try { await StartQueue(); }
    catch (e: any) { pushToast({ kind: 'error', title: 'Không thể bắt đầu', body: String(e?.message ?? e) }); }
  }

  return (
    <div className="flex items-center gap-3 rounded-xl surface-panel px-4 py-3">
      {queueState === 'idle' && (
        <Button size="lg" variant="success" onClick={start} disabled={pendingCount === 0}>
          <Play size={16} /> Bắt đầu
        </Button>
      )}
      {queueState === 'running' && (
        <>
          <Button size="lg" variant="secondary" onClick={() => PauseQueue()}>
            <Pause size={16} /> Tạm dừng
          </Button>
          <Button size="lg" variant="danger" onClick={() => StopQueue()}>
            <Square size={16} /> Dừng
          </Button>
        </>
      )}
      {queueState === 'paused' && (
        <>
          <Button size="lg" variant="success" onClick={() => ResumeQueue()}>
            <Play size={16} /> Tiếp tục
          </Button>
          <Button size="lg" variant="danger" onClick={() => StopQueue()}>
            <Square size={16} /> Dừng
          </Button>
        </>
      )}
      {queueState === 'stopping' && (
        <Button size="lg" disabled>
          <Loader2 size={16} className="animate-spin" /> Đang dừng…
        </Button>
      )}
      <div className="flex-1" />
      <span className="text-xs text-muted">{pendingCount} prompt chờ xử lý</span>
    </div>
  );
}
