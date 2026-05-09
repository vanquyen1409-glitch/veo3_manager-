import { EventsOn } from '../../wailsjs/runtime/runtime';
import { useAppStore } from './appStore';
import { useQueueStore } from './queueStore';
import type { browser } from '../../wailsjs/go/models';

let installed = false;

export function installEventBridge() {
  if (installed) return;
  installed = true;

  EventsOn('browserStatus', (snap: browser.Snapshot) => {
    useAppStore.getState().setBrowser(snap);
  });

  EventsOn('queue:state', (p: { state: string; message?: string }) => {
    useAppStore.getState().setQueueState(p.state as any);
  });

  EventsOn('queue:task', (e: { id: string; phase: string; patch?: any }) => {
    useQueueStore.getState().applyTaskEvent(e);
    if (e.phase === 'succeeded') {
      useAppStore.getState().pushToast({
        kind: 'success',
        title: 'Video ready',
        body: e.patch?.paths?.length
          ? `${e.patch.paths.length} file(s) saved`
          : undefined,
      });
    } else if (e.phase === 'failed') {
      useAppStore.getState().pushToast({
        kind: 'error',
        title: 'Generation failed',
        body: e.patch?.error,
        timeoutMs: 6000,
      });
    } else if (e.phase === 'cancelled') {
      useAppStore.getState().pushToast({
        kind: 'warn',
        title: 'Task cancelled',
      });
    }
  });

  EventsOn('queue:stats', () => {
    // Dashboard listens directly via its own subscription if needed.
  });
}
