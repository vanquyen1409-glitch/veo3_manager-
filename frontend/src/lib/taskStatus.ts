// Shared task-status / queue-phase formatting. The queue worker emits more
// granular phases than the DB stores (submitted/polling/downloading) so the
// "phase" label superset includes the DB statuses too.

export const TASK_PHASE_LABEL: Record<string, string> = {
  pending: 'chờ xử lý',
  running: 'đang chạy',
  submitted: 'đã gửi',
  polling: 'đang chờ',
  downloading: 'đang tải',
  succeeded: 'thành công',
  failed: 'thất bại',
  cancelled: 'đã hủy',
};

export const TASK_PHASE_TONE: Record<string, string> = {
  pending: 'text-slate-500 dark:text-slate-400',
  running: 'text-amber-600 dark:text-amber-400',
  submitted: 'text-amber-600 dark:text-amber-400',
  polling: 'text-amber-600 dark:text-amber-400',
  downloading: 'text-accent-700 dark:text-accent-400',
  succeeded: 'text-emerald-600 dark:text-emerald-400',
  failed: 'text-red-600 dark:text-red-400',
  cancelled: 'text-slate-500',
};

export const TASK_STATUS_BADGE: Record<string, string> = {
  succeeded: 'bg-emerald-100 text-emerald-800 border-emerald-300 dark:bg-emerald-900/40 dark:text-emerald-300 dark:border-emerald-700',
  failed:    'bg-red-100 text-red-800 border-red-300 dark:bg-red-900/40 dark:text-red-300 dark:border-red-700',
  cancelled: 'bg-slate-100 text-slate-600 border-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700',
  running:   'bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-900/40 dark:text-amber-300 dark:border-amber-700',
  pending:   'bg-slate-100 text-slate-600 border-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700',
};

const DEFAULT_BADGE = 'bg-slate-100 text-slate-600 border-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700';

export function taskPhaseLabel(phase: string | undefined): string {
  return TASK_PHASE_LABEL[phase ?? ''] ?? (phase ?? '');
}

export function taskPhaseTone(phase: string | undefined): string {
  return TASK_PHASE_TONE[phase ?? ''] ?? 'text-slate-400';
}

export function taskStatusBadge(status: string | undefined): string {
  return TASK_STATUS_BADGE[status ?? ''] ?? DEFAULT_BADGE;
}
