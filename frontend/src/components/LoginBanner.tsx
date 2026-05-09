import { ShieldCheck, AlertCircle } from 'lucide-react';
import { useAppStore } from '../store/appStore';
import { Button } from './ui/Button';
import { OpenSafeLogin } from '../../wailsjs/go/main/App';

const BANNER_LABEL: Record<string, string> = {
  needs_login: 'Cần đăng nhập Google',
  connecting: 'Đang kết nối Chrome…',
  disconnected: 'Chưa kết nối Chrome',
  error: 'Lỗi kết nối Chrome',
};

/**
 * Prominent banner shown on every page when Chrome is not yet authenticated.
 * Uses the SAFE login path (no CDP) so Google doesn't block with
 * "This browser or app may not be secure".
 */
export default function LoginBanner() {
  const browser = useAppStore((s) => s.browser);
  const pushToast = useAppStore((s) => s.pushToast);
  const status = browser?.status ?? 'disconnected';

  if (status === 'ready') return null;

  const isError = status === 'error';
  const tone = isError
    ? 'border-red-700/60 bg-red-500/10 text-red-200'
    : 'border-amber-700/60 bg-amber-500/10 text-amber-200';
  const iconTone = isError ? 'text-red-400' : 'text-amber-400';

  async function open() {
    try {
      await OpenSafeLogin();
      pushToast({
        kind: 'info',
        title: 'Đã mở Chrome an toàn',
        body: 'Đăng nhập Gmail trong cửa sổ Chrome rồi đóng cửa sổ — app sẽ tự kết nối lại.',
        timeoutMs: 8000,
      });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
    }
  }

  return (
    <div className={`flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 ${tone}`}>
      <AlertCircle size={18} className={iconTone} />
      <div className="flex-1 text-sm">
        <b>{BANNER_LABEL[status] ?? 'Chưa sẵn sàng'}.</b>{' '}
        <span className="opacity-80">
          Bấm "Đăng nhập an toàn" để mở Chrome (chế độ tránh detection của Google) và đăng nhập Gmail một lần.
        </span>
      </div>
      <Button size="sm" variant="success" onClick={open}>
        <ShieldCheck size={14} /> Đăng nhập an toàn
      </Button>
    </div>
  );
}
