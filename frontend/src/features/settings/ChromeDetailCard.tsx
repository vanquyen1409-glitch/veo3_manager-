import { useEffect, useState } from 'react';
import { RefreshCw, Cable } from 'lucide-react';
import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { GetBrowserDetail } from '../../../wailsjs/go/main/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import type { browser } from '../../../wailsjs/go/models';
import { useAppStore } from '../../store/appStore';
import { copyToClipboard } from '../../lib/copy';
import { cn } from '../../lib/cn';
import ChromeConnectionPanel from './ChromeConnectionPanel';
import ChromeProfilePanel from './ChromeProfilePanel';
import { Skeleton } from '../../components/ui/Skeleton';

export default function ChromeDetailCard() {
  const [detail, setDetail] = useState<browser.Detail | null>(null);
  const [busy, setBusy] = useState(false);
  const pushToast = useAppStore((s) => s.pushToast);

  async function refresh() {
    setBusy(true);
    try {
      const next = await GetBrowserDetail();
      setDetail(next as any);
    } finally {
      setBusy(false);
    }
  }

  // Event-driven only: every browserStatus emit (transition, token refresh,
  // reset, etc) already triggers a refresh, so a 10s interval was redundant
  // and doubled the IPC traffic on every reconnect.
  useEffect(() => {
    refresh();
    const off = EventsOn('browserStatus', () => refresh());
    return () => {
      off?.();
    };
  }, []);

  async function copy(text: string, label: string) {
    if (await copyToClipboard(text)) {
      pushToast({ kind: 'success', title: `Đã copy ${label}` });
      return;
    }
    pushToast({ kind: 'error', title: `Copy thất bại` });
  }

  if (!detail) {
    return (
      <Card>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2" aria-busy="true" aria-label="Đang tải thông tin Chrome">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="flex flex-col gap-1.5">
              <Skeleton className="h-2.5 w-24" />
              <Skeleton className="h-9 w-full" />
            </div>
          ))}
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Cable size={16} className="text-accent-400" /> Chrome — chi tiết kết nối
        </CardTitle>
        <Button size="sm" variant="ghost" onClick={refresh} disabled={busy}>
          <RefreshCw size={13} className={cn(busy && 'animate-spin')} /> Làm mới
        </Button>
      </CardHeader>

      <ChromeConnectionPanel detail={detail} onCopy={copy} />
      <ChromeProfilePanel detail={detail} onCopy={copy} />
    </Card>
  );
}
