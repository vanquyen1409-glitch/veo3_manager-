// Shared browser-connection status formatting. The Snapshot.status field has
// 5 known values plus an unknown fallback. Any UI that paints the dot or
// shows a label should read from these maps so they stay in sync.

export const BROWSER_STATUS_DOT_COLOR: Record<string, string> = {
  ready: 'bg-emerald-500',
  connecting: 'bg-amber-400',
  needs_login: 'bg-amber-400',
  disconnected: 'bg-red-500',
  error: 'bg-red-500',
};

export const BROWSER_STATUS_LABEL: Record<string, string> = {
  ready: 'Đã kết nối',
  connecting: 'Đang kết nối…',
  needs_login: 'Cần đăng nhập',
  disconnected: 'Chưa kết nối',
  error: 'Lỗi',
};

export function browserStatusDotColor(status: string | undefined): string {
  return BROWSER_STATUS_DOT_COLOR[status ?? ''] ?? 'bg-slate-400 dark:bg-slate-500';
}

export function browserStatusLabel(status: string | undefined): string {
  return BROWSER_STATUS_LABEL[status ?? ''] ?? (status ?? '');
}
