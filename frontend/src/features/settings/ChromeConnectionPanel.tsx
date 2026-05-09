import { Copy, ExternalLink, Cpu, Globe, Layers } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import type { browser } from '../../../wailsjs/go/models';
import { formatDate } from '../../lib/format';
import { cn } from '../../lib/cn';
import { browserStatusDotColor } from '../../lib/browserStatus';
import { Field, truncateMid } from './ChromeDetailField';

interface Props {
  detail: browser.Detail;
  onCopy: (text: string, label: string) => void;
}

export default function ChromeConnectionPanel({ detail, onCopy }: Props) {
  const hasConn = !!detail.wsUrl;
  const status = detail.status;

  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
      <Field
        label="Trạng thái"
        value={
          <div className="flex items-center gap-2">
            <span className={cn('inline-flex h-2 w-2 rounded-full', browserStatusDotColor(status))} />
            <span>{status}</span>
            {detail.message && <span className="text-subtle">· {detail.message}</span>}
          </div>
        }
      />
      <Field
        label="Chế độ kết nối"
        value={
          detail.weOwn
            ? 'App tự khởi động Chrome (we own)'
            : hasConn
              ? 'Tái sử dụng Chrome đang chạy (reused)'
              : 'Chưa kết nối'
        }
      />

      <Field
        label="WebSocket Debugger URL (CDP)"
        mono
        value={detail.wsUrl ? truncateMid(detail.wsUrl, 60) : '—'}
        actions={
          detail.wsUrl && (
            <Button size="sm" variant="secondary" onClick={() => onCopy(detail.wsUrl!, 'WS URL')}>
              <Copy size={13} />
            </Button>
          )
        }
      />
      <Field
        label="JSON Version endpoint"
        mono
        value={detail.jsonVersionUrl || '—'}
        actions={
          detail.jsonVersionUrl && (
            <>
              <Button size="sm" variant="secondary" onClick={() => onCopy(detail.jsonVersionUrl!, 'URL')}>
                <Copy size={13} />
              </Button>
              <Button size="sm" variant="ghost" onClick={() => BrowserOpenURL(detail.jsonVersionUrl!)}>
                <ExternalLink size={13} />
              </Button>
            </>
          )
        }
      />

      <Field label="Debug port" value={detail.debugPort ? String(detail.debugPort) : '—'} />
      <Field label="Chrome PID" value={detail.pid ? String(detail.pid) : '—'} />

      <Field
        label={
          <span className="inline-flex items-center gap-1.5">
            <Cpu size={12} /> Chrome version
          </span>
        }
        value={detail.chromeVersion || '—'}
      />
      <Field
        label={
          <span className="inline-flex items-center gap-1.5">
            <Layers size={12} /> Tabs đang mở
          </span>
        }
        value={detail.profileTabs ? `${detail.profileTabs} tab` : '—'}
      />

      <Field
        label={
          <span className="inline-flex items-center gap-1.5">
            <Globe size={12} /> User-Agent
          </span>
        }
        mono
        value={detail.userAgent || '—'}
      />
      <Field
        label="Kết nối lúc"
        value={detail.connectedAtMs ? formatDate(new Date(detail.connectedAtMs)) : '—'}
      />
    </div>
  );
}
