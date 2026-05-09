import { useState } from 'react';
import {
  Chrome as ChromeIcon,
  LogIn,
  ShieldCheck,
  RefreshCw,
  FolderOpen,
  Trash2,
  CheckCircle2,
  AlertCircle,
} from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { useAppStore } from '../../store/appStore';
import {
  OpenLoginFlow,
  OpenSafeLogin,
  RefreshToken,
  ResetBrowserProfile,
  OpenProfileFolder,
} from '../../../wailsjs/go/main/App';
import { cn } from '../../lib/cn';
import { formatDuration } from '../../lib/format';

const stateMeta: Record<
  string,
  { tone: string; bg: string; icon: typeof CheckCircle2; title: string; help: string }
> = {
  ready: {
    tone: 'text-emerald-600 dark:text-emerald-400',
    bg: 'bg-emerald-500/15 border-emerald-400/60 dark:border-emerald-700/50',
    icon: CheckCircle2,
    title: 'Đã đăng nhập Google',
    help: 'Token đã được lưu. Bạn có thể bắt đầu tạo video.',
  },
  needs_login: {
    tone: 'text-amber-600 dark:text-amber-400',
    bg: 'bg-amber-500/15 border-amber-400/60 dark:border-amber-700/50',
    icon: AlertCircle,
    title: 'Cần đăng nhập Google',
    help: 'Nếu Gmail báo "This browser may not be secure" — dùng nút "Đăng nhập an toàn" bên dưới (mở Chrome KHÔNG có CDP, Google sẽ không chặn).',
  },
  connecting: {
    tone: 'text-amber-600 dark:text-amber-400',
    bg: 'bg-amber-500/15 border-amber-400/60 dark:border-amber-700/50',
    icon: AlertCircle,
    title: 'Đang kết nối Chrome…',
    help: 'Vui lòng chờ. Nếu lâu, kiểm tra Chrome đã cài và port debug không bị chiếm.',
  },
  disconnected: {
    tone: 'text-red-600 dark:text-red-400',
    bg: 'bg-red-500/15 border-red-400/60 dark:border-red-700/50',
    icon: AlertCircle,
    title: 'Chưa kết nối Chrome',
    help: 'Nhấn "Đăng nhập an toàn" để khởi động Chrome với profile riêng (Google không phát hiện automation).',
  },
  error: {
    tone: 'text-red-600 dark:text-red-400',
    bg: 'bg-red-500/15 border-red-400/60 dark:border-red-700/50',
    icon: AlertCircle,
    title: 'Lỗi kết nối Chrome',
    help: 'Xem chi tiết bên dưới và thử lại.',
  },
};

export default function AccountCard() {
  const browser = useAppStore((s) => s.browser);
  const pushToast = useAppStore((s) => s.pushToast);
  const [busy, setBusy] = useState(false);

  const status = browser?.status ?? 'disconnected';
  const meta = stateMeta[status] ?? stateMeta.disconnected;
  const Icon = meta.icon;

  async function safeLogin() {
    setBusy(true);
    try {
      await OpenSafeLogin();
      pushToast({
        kind: 'info',
        title: 'Đã mở Chrome an toàn',
        body: 'Đăng nhập Gmail rồi đóng Chrome. App sẽ tự kết nối lại với cookies vừa lưu.',
        timeoutMs: 8000,
      });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
    } finally {
      setBusy(false);
    }
  }

  async function normalLogin() {
    setBusy(true);
    try {
      await OpenLoginFlow();
      pushToast({
        kind: 'info',
        title: 'Đã mở Chrome (CDP mode)',
        body: 'Chế độ này có debug socket cho external automation.',
      });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
    } finally {
      setBusy(false);
    }
  }

  async function refresh() {
    setBusy(true);
    try {
      await RefreshToken();
      pushToast({ kind: 'success', title: 'Đã làm mới token' });
    } catch (e: any) {
      pushToast({ kind: 'warn', title: 'Chưa lấy được token', body: String(e?.message ?? e) });
    } finally {
      setBusy(false);
    }
  }

  const openConfirm = useAppStore((s) => s.openConfirm);

  function reset() {
    openConfirm({
      title: 'Xóa profile Chrome?',
      body: 'Cookies + login Gmail sẽ mất, bạn phải đăng nhập lại từ đầu.',
      confirmLabel: 'Xóa',
      danger: true,
      onConfirm: async () => {
        setBusy(true);
        try {
          await ResetBrowserProfile();
          pushToast({ kind: 'success', title: 'Đã xóa profile' });
        } catch (e: any) {
          pushToast({ kind: 'error', title: 'Lỗi', body: String(e?.message ?? e) });
        } finally {
          setBusy(false);
        }
      },
    });
  }

  return (
    <Card>
      <div className="flex items-start gap-4">
        <span className={cn('flex h-12 w-12 flex-none items-center justify-center rounded-xl border', meta.bg)}>
          <ChromeIcon size={22} className={meta.tone} />
        </span>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <Icon size={16} className={meta.tone} />
            <h2 className="text-strong text-base font-semibold">{meta.title}</h2>
          </div>
          <p className="text-muted mt-1 text-sm">{meta.help}</p>
          {browser?.tokenAgeMs && status === 'ready' && (
            <p className="mt-2 text-xs text-subtle">
              Token cách đây {formatDuration(browser.tokenAgeMs)} · cookies tự động lưu
            </p>
          )}
        </div>
      </div>

      {status !== 'ready' && (
        <div className="mt-4 rounded-md border border-emerald-400/60 dark:border-emerald-700/40 bg-emerald-500/10 p-3 text-xs text-emerald-800 dark:text-emerald-200">
          <b>💡 Khuyên dùng:</b> Bấm <b>"Đăng nhập an toàn"</b> nếu Google chặn login với cảnh báo
          <i> "This browser or app may not be secure"</i>. App sẽ mở Chrome chuẩn (không có CDP),
          bạn login Gmail xong đóng cửa sổ — cookies sẽ được lưu vào profile và app tự kết nối lại.
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-2">
        {status !== 'ready' ? (
          <>
            <Button size="md" variant="success" onClick={safeLogin} disabled={busy}>
              <ShieldCheck size={14} /> Đăng nhập an toàn (khuyên dùng)
            </Button>
            <Button size="md" variant="secondary" onClick={normalLogin} disabled={busy}>
              <LogIn size={14} /> Mở Chrome CDP
            </Button>
          </>
        ) : (
          <Button size="md" variant="secondary" onClick={normalLogin} disabled={busy}>
            <ChromeIcon size={14} /> Mở Chrome
          </Button>
        )}
        <Button size="md" variant="ghost" onClick={refresh} disabled={busy}>
          <RefreshCw size={14} /> Kiểm tra lại
        </Button>
        <Button size="md" variant="ghost" onClick={() => OpenProfileFolder().catch(() => {})}>
          <FolderOpen size={14} /> Mở thư mục profile
        </Button>
        <div className="flex-1" />
        <Button size="md" variant="danger" onClick={reset} disabled={busy}>
          <Trash2 size={14} /> Xóa profile
        </Button>
      </div>
    </Card>
  );
}
