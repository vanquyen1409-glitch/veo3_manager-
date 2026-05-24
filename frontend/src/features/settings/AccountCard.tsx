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
  ChevronDown,
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
    help: 'Làm theo 4 bước bên dưới để đăng nhập an toàn, Google sẽ không chặn.',
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
    help: 'Làm theo 4 bước bên dưới để đăng nhập an toàn, Google sẽ không chặn.',
  },
  error: {
    tone: 'text-red-600 dark:text-red-400',
    bg: 'bg-red-500/15 border-red-400/60 dark:border-red-700/50',
    icon: AlertCircle,
    title: 'Lỗi kết nối Chrome',
    help: 'Xem chi tiết bên dưới và thử lại.',
  },
};

const STEPS = [
  'Bấm nút “Đăng nhập an toàn” — app mở cửa sổ Chrome thật.',
  'Đăng nhập Gmail như bình thường trong cửa sổ đó.',
  'Đăng nhập xong thì đóng cửa sổ Chrome lại.',
  'App tự kết nối lại bằng phiên vừa lưu — xong!',
];

export default function AccountCard() {
  const browser = useAppStore((s) => s.browser);
  const pushToast = useAppStore((s) => s.pushToast);
  const [busy, setBusy] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const status = browser?.status ?? 'disconnected';
  const meta = stateMeta[status] ?? stateMeta.disconnected;
  const Icon = meta.icon;
  const ready = status === 'ready';

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
      {/* Status header */}
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
          {browser?.tokenAgeMs && ready && (
            <p className="mt-2 text-xs text-subtle">
              Token cách đây {formatDuration(browser.tokenAgeMs)} · cookies tự động lưu
            </p>
          )}
        </div>
      </div>

      {!ready && (
        <>
          {/* 4-step visual guide */}
          <ol className="mt-4 space-y-2.5 rounded-lg border border-emerald-400/50 dark:border-emerald-700/40 bg-emerald-500/[0.07] p-4">
            {STEPS.map((text, i) => (
              <li key={i} className="flex items-start gap-3 text-sm text-emerald-900 dark:text-emerald-100">
                <span className="flex h-5 w-5 flex-none items-center justify-center rounded-full bg-emerald-600 text-[11px] font-bold text-white">
                  {i + 1}
                </span>
                <span className="leading-5">{text}</span>
              </li>
            ))}
          </ol>

          {/* Primary action */}
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <Button size="lg" variant="success" onClick={safeLogin} disabled={busy}>
              <ShieldCheck size={16} /> Đăng nhập an toàn
            </Button>
            <Button size="lg" variant="ghost" onClick={refresh} disabled={busy}>
              <RefreshCw size={14} /> Kiểm tra lại
            </Button>
          </div>
        </>
      )}

      {ready && (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button size="md" variant="secondary" onClick={normalLogin} disabled={busy}>
            <ChromeIcon size={14} /> Mở Chrome
          </Button>
          <Button size="md" variant="ghost" onClick={refresh} disabled={busy}>
            <RefreshCw size={14} /> Kiểm tra lại
          </Button>
        </div>
      )}

      {/* Advanced — tucked away so it can't be clicked by mistake */}
      <div className="mt-4 border-t border-[var(--border)] pt-3">
        <button
          type="button"
          onClick={() => setShowAdvanced((v) => !v)}
          className="flex items-center gap-1.5 text-xs font-medium text-subtle transition-colors hover:text-soft"
        >
          <ChevronDown size={14} className={cn('transition-transform', showAdvanced && 'rotate-180')} />
          Tùy chọn nâng cao
        </button>

        {showAdvanced && (
          <div className="mt-3 flex flex-wrap items-center gap-2">
            {!ready && (
              <Button size="sm" variant="secondary" onClick={normalLogin} disabled={busy}>
                <LogIn size={14} /> Mở Chrome CDP
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={() => OpenProfileFolder().catch(() => {})}>
              <FolderOpen size={14} /> Mở thư mục profile
            </Button>
            <div className="flex-1" />
            <Button size="sm" variant="danger" onClick={reset} disabled={busy}>
              <Trash2 size={14} /> Xóa profile
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
}
