import { useEffect, useState } from 'react';
import ConfigBar from '../features/queue/ConfigBar';
import PromptInput from '../features/queue/PromptInput';
import TaskList from '../features/queue/TaskList';
import ControlBar from '../features/queue/ControlBar';
import { useQueueStore } from '../store/queueStore';
import { useAppStore } from '../store/appStore';
import { GetSettings } from '../../wailsjs/go/main/App';
import type { db } from '../../wailsjs/go/models';

const fallbackCfg: db.GenerationConfig = {
  model: 'veo_3_1_t2v_fast',
  aspectRatio: '16:9',
  outputCount: 1,
  seedBase: 0,
  outputDir: '',
} as unknown as db.GenerationConfig;

export default function QueuePage() {
  const [cfg, setCfg] = useState<db.GenerationConfig>(fallbackCfg);
  const enqueue = useQueueStore((s) => s.enqueue);
  const loadInitial = useQueueStore((s) => s.loadInitial);
  const pushToast = useAppStore((s) => s.pushToast);

  // Mount-only: GetSettings + loadInitial run exactly once. Listing `loaded`
  // in deps used to fire the effect a second time when loadInitial flipped
  // the flag, doubling the IPC call to GetSettings on every page open.
  useEffect(() => {
    let cancelled = false;
    GetSettings()
      .then((b) => {
        if (cancelled) return;
        setCfg({
          ...b.defaultConfig,
          outputDir: b.defaultConfig.outputDir || b.outputDir,
        });
      })
      .catch(() => {});
    if (!useQueueStore.getState().loaded) {
      loadInitial().catch(() => {});
    }
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleEnqueue(prompts: string[]) {
    try {
      const created = await enqueue(prompts, cfg);
      pushToast({
        kind: 'success',
        title: `Đã thêm ${created.length} prompt vào hàng đợi`,
      });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Thêm thất bại', body: String(e?.message ?? e) });
    }
  }

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <ConfigBar cfg={cfg} onChange={setCfg} />

      <div className="grid flex-1 grid-cols-1 gap-4 overflow-hidden lg:grid-cols-2">
        <PromptInput onEnqueue={handleEnqueue} />
        <TaskList />
      </div>

      <ControlBar />
    </div>
  );
}
