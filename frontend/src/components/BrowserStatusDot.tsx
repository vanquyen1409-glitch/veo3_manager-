import { useAppStore } from '../store/appStore';
import { cn } from '../lib/cn';
import { OpenSafeLogin } from '../../wailsjs/go/main/App';
import { browserStatusDotColor, browserStatusLabel } from '../lib/browserStatus';

export default function BrowserStatusDot() {
  const browser = useAppStore((s) => s.browser);
  const pushToast = useAppStore((s) => s.pushToast);
  const status = browser?.status ?? 'disconnected';
  const isReady = status === 'ready';

  async function onClick() {
    if (isReady) return;
    try {
      await OpenSafeLogin();
      pushToast({ kind: 'info', title: 'Đã mở Chrome an toàn — đăng nhập rồi đóng cửa sổ' });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
    }
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={isReady}
      title={browser?.message || browserStatusLabel(status)}
      className={cn(
        'text-soft flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs transition',
        isReady ? 'cursor-default' : 'hover-surface',
      )}
    >
      <span className={cn('h-2 w-2 flex-none rounded-full animate-pulse-slow', browserStatusDotColor(status))} />
      <span>Chrome</span>
      <span className="text-subtle ml-auto truncate text-[11px]">{browserStatusLabel(status)}</span>
    </button>
  );
}
