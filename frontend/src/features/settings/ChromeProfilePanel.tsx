import { Copy, FolderOpen, HardDrive, Mail } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { OpenProfileFolder } from '../../../wailsjs/go/main/App';
import type { browser } from '../../../wailsjs/go/models';
import { formatBytes } from '../../lib/copy';
import { Field } from './ChromeDetailField';

interface Props {
  detail: browser.Detail;
  onCopy: (text: string, label: string) => void;
}

export default function ChromeProfilePanel({ detail, onCopy }: Props) {
  return (
    <div className="mt-5 border-t divider pt-4">
      <div className="label mb-2">Profile Chrome (cookies, login, lịch sử)</div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <Field
          label="Đường dẫn"
          mono
          value={detail.userDataDir || '—'}
          actions={
            detail.userDataDir && (
              <>
                <Button size="sm" variant="secondary" onClick={() => onCopy(detail.userDataDir!, 'đường dẫn')}>
                  <Copy size={13} />
                </Button>
                <Button size="sm" variant="ghost" onClick={() => OpenProfileFolder().catch(() => {})}>
                  <FolderOpen size={13} />
                </Button>
              </>
            )
          }
        />
        <Field
          label={
            <span className="inline-flex items-center gap-1.5">
              <HardDrive size={12} /> Dung lượng profile
            </span>
          }
          value={formatBytes(detail.profileSizeBytes)}
        />
        <Field
          label={
            <span className="inline-flex items-center gap-1.5">
              <Mail size={12} /> Tài khoản đăng nhập
            </span>
          }
          value={
            detail.loginEmail ? (
              <span className="text-emerald-700 dark:text-emerald-300">{detail.loginEmail}</span>
            ) : detail.status === 'ready' ? (
              <span className="text-subtle">Đã có token (không lấy được email)</span>
            ) : (
              <span className="text-amber-700 dark:text-amber-400">Chưa đăng nhập</span>
            )
          }
        />
        <Field
          label="Token Google (Bearer)"
          value={
            detail.tokenAgeMs && detail.tokenAgeMs > 0
              ? `Đã có (lấy cách đây ${Math.round(detail.tokenAgeMs / 1000)}s)`
              : 'Chưa có'
          }
        />
      </div>
    </div>
  );
}
