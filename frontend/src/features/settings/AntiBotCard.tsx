import { useEffect, useState } from 'react';
import { ShieldCheck, Check } from 'lucide-react';
import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import { GetBrowserDetail } from '../../../wailsjs/go/main/App';
import type { browser } from '../../../wailsjs/go/models';

export default function AntiBotCard() {
  const [d, setD] = useState<browser.Detail | null>(null);

  useEffect(() => {
    GetBrowserDetail().then((x) => setD(x as any)).catch(() => {});
  }, []);

  const flags = d?.bypassFlags ?? [];
  const patches = d?.stealthPatches ?? [];
  const enabled = d?.stealthEnabled ?? true;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ShieldCheck size={16} className={enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'} />
          Chống phát hiện bot (anti-bot bypass)
        </CardTitle>
        <span className={`rounded-full border px-2 py-0.5 text-[11px] ${enabled ? 'border-emerald-400 dark:border-emerald-700 bg-emerald-500/15 text-emerald-700 dark:text-emerald-300' : 'border-amber-400 dark:border-amber-700 bg-amber-500/15 text-amber-700 dark:text-amber-300'}`}>
          {enabled ? 'Đang bật' : 'Đã tắt'}
        </span>
      </CardHeader>

      <p className="mb-4 text-xs text-muted">
        App áp dụng các flag launch + script stealth để Google Labs không phát hiện đây là Chrome
        điều khiển bằng automation. Nếu thiếu lớp này, request token sẽ bị từ chối.
      </p>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <div className="label mb-2">Chrome launch flags</div>
          <ul className="flex flex-col gap-1.5 text-xs">
            {flags.map((f) => (
              <li key={f} className="flex items-start gap-2">
                <Check size={13} className="mt-0.5 flex-none text-emerald-600 dark:text-emerald-400" />
                <code className="text-soft font-mono">{f}</code>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <div className="label mb-2">Stealth patches (go-rod/stealth)</div>
          <ul className="flex flex-col gap-1.5 text-xs">
            {patches.map((p) => (
              <li key={p} className="flex items-start gap-2">
                <Check size={13} className="mt-0.5 flex-none text-emerald-600 dark:text-emerald-400" />
                <span className="text-soft">{p}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Card>
  );
}
