import { create } from 'zustand';
import type { browser } from '../../wailsjs/go/models';

export type BrowserStatus =
  | 'disconnected'
  | 'connecting'
  | 'needs_login'
  | 'ready'
  | 'error';

export type QueueState = 'idle' | 'running' | 'paused' | 'stopping';

export type ToastKind = 'info' | 'success' | 'warn' | 'error';

export interface Toast {
  id: string;
  kind: ToastKind;
  title: string;
  body?: string;
  timeoutMs?: number;
}

export interface ConfirmRequest {
  id: string;
  title: string;
  body?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void | Promise<void>;
}

interface AppState {
  browser: browser.Snapshot;
  queueState: QueueState;
  toasts: Toast[];
  confirm: ConfirmRequest | null;
  setBrowser: (s: browser.Snapshot) => void;
  setQueueState: (s: QueueState) => void;
  pushToast: (t: Omit<Toast, 'id'>) => void;
  dismissToast: (id: string) => void;
  openConfirm: (req: Omit<ConfirmRequest, 'id'>) => void;
  resolveConfirm: () => void;
}

const newId = () =>
  globalThis.crypto?.randomUUID?.() ?? Math.random().toString(36).slice(2);

export const useAppStore = create<AppState>((set, get) => ({
  browser: { status: 'disconnected' },
  queueState: 'idle',
  toasts: [],
  confirm: null,
  setBrowser: (s) => set({ browser: s }),
  setQueueState: (s) => set({ queueState: s }),
  pushToast: (t) => {
    const id = newId();
    const toast: Toast = { id, ...t };
    set((s) => ({ toasts: [...s.toasts, toast] }));
    const ms = t.timeoutMs ?? 4000;
    if (ms > 0) {
      setTimeout(() => get().dismissToast(id), ms);
    }
  },
  dismissToast: (id) =>
    set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) })),
  openConfirm: (req) => set({ confirm: { id: newId(), ...req } }),
  resolveConfirm: () => set({ confirm: null }),
}));
