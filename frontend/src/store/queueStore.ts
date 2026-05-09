import { create } from 'zustand';
import type { db } from '../../wailsjs/go/models';
import {
  ListTasks,
  EnqueuePrompts,
  RequeueTask,
  DeleteTask,
} from '../../wailsjs/go/main/App';

export type TaskPhase =
  | 'pending'
  | 'running'
  | 'submitted'
  | 'polling'
  | 'downloading'
  | 'succeeded'
  | 'failed'
  | 'cancelled';

// db.Task is generated as a class with `convertValues`. We only need the data
// shape, so omit the helper to make plain-object construction valid.
export type TaskRow = Omit<db.Task, 'convertValues'> & {
  phase?: TaskPhase;
  progress?: { i?: number; n?: number };
};

interface QState {
  tasks: Record<string, TaskRow>;
  loaded: boolean;
  loadInitial: () => Promise<void>;
  applyTaskEvent: (e: { id: string; phase: string; patch?: Record<string, any> }) => void;
  enqueue: (prompts: string[], cfg: db.GenerationConfig) => Promise<TaskRow[]>;
  requeue: (id: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
}

export const useQueueStore = create<QState>((set) => ({
  tasks: {},
  loaded: false,
  loadInitial: async () => {
    const list = await ListTasks({
      statuses: ['pending', 'running'],
      limit: 200,
    } as unknown as db.ListFilter);
    const map: Record<string, TaskRow> = {};
    for (const t of list) map[t.id] = t as TaskRow;
    set({ tasks: map, loaded: true });
  },
  applyTaskEvent: (e) => {
    set((s) => {
      const cur = s.tasks[e.id] ?? ({ id: e.id } as TaskRow);
      const next: TaskRow = { ...cur, phase: e.phase as TaskPhase };
      const patch = e.patch ?? {};
      if (typeof patch.error === 'string') next.error = patch.error;
      if (Array.isArray(patch.paths)) next.videoPaths = patch.paths;
      if (typeof patch.i === 'number' || typeof patch.n === 'number') {
        next.progress = { i: patch.i, n: patch.n };
      }
      // Map phases to status enum so UI badges match.
      if (e.phase === 'succeeded') next.status = 'succeeded';
      else if (e.phase === 'failed') next.status = 'failed';
      else if (e.phase === 'cancelled') next.status = 'cancelled';
      else if (e.phase === 'running' || e.phase === 'submitted'
            || e.phase === 'polling' || e.phase === 'downloading') {
        next.status = 'running';
      }
      return { tasks: { ...s.tasks, [e.id]: next } };
    });
  },
  enqueue: async (prompts, cfg) => {
    const created = (await EnqueuePrompts(prompts, cfg)) as TaskRow[];
    set((s) => {
      const next = { ...s.tasks };
      for (const t of created) next[t.id] = t;
      return { tasks: next };
    });
    return created;
  },
  requeue: async (id) => {
    const t = (await RequeueTask(id)) as TaskRow;
    set((s) => ({ tasks: { ...s.tasks, [t.id]: t } }));
  },
  remove: async (id) => {
    await DeleteTask(id);
    set((s) => {
      const next = { ...s.tasks };
      delete next[id];
      return { tasks: next };
    });
  },
}));
