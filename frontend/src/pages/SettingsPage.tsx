import { useEffect, useState } from 'react';
import { Folder, Save, Chrome as ChromeIcon, Video } from 'lucide-react';
import { Card, CardHeader, CardTitle } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { useAppStore } from '../store/appStore';
import {
  GetSettings,
  SaveSettings,
  ChooseDirectory,
  Version,
} from '../../wailsjs/go/main/App';
import type { db } from '../../wailsjs/go/models';
import AccountCard from '../features/settings/AccountCard';
import ChromeDetailCard from '../features/settings/ChromeDetailCard';
import AntiBotCard from '../features/settings/AntiBotCard';
import { SkeletonCard } from '../components/ui/Skeleton';

export default function SettingsPage() {
  const [bundle, setBundle] = useState<db.SettingsBundle | null>(null);
  const [version, setVersion] = useState('1.0.0');
  const pushToast = useAppStore((s) => s.pushToast);

  useEffect(() => {
    GetSettings().then(setBundle).catch(() => {});
    Version().then(setVersion).catch(() => {});
  }, []);

  if (!bundle) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  const update = (patch: Partial<db.SettingsBundle>) =>
    setBundle((b) => (b ? ({ ...b, ...patch } as db.SettingsBundle) : b));

  async function browse(field: 'userDataDir' | 'outputDir') {
    const dir = await ChooseDirectory((bundle as any)?.[field] ?? '');
    if (dir) update({ [field]: dir } as any);
  }

  async function save() {
    try {
      await SaveSettings(bundle!);
      pushToast({ kind: 'success', title: 'Đã lưu cài đặt' });
    } catch (e: any) {
      pushToast({ kind: 'error', title: 'Lưu thất bại', body: String(e?.message ?? e) });
    }
  }

  return (
    <div className="flex flex-col gap-4 p-6">
      <h1>Cài đặt</h1>

      <AccountCard />
      <ChromeDetailCard />
      <AntiBotCard />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ChromeIcon size={16} className="text-accent-400" /> Chrome
            </CardTitle>
          </CardHeader>
          <div className="flex flex-col gap-3">
            <FieldFolder
              label="Chrome User Data"
              value={bundle.userDataDir}
              onChange={(v) => update({ userDataDir: v })}
              onBrowse={() => browse('userDataDir')}
            />
            <Field
              label="Debug Port"
              value={String(bundle.chromeDebugPort)}
              onChange={(v) => update({ chromeDebugPort: Number(v) || 9222 })}
            />
            <FieldFolder
              label="Thư mục tải video"
              value={bundle.outputDir}
              onChange={(v) => update({ outputDir: v })}
              onBrowse={() => browse('outputDir')}
            />
          </div>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Video size={16} className="text-accent-400" /> Tạo video
            </CardTitle>
          </CardHeader>
          <div className="flex flex-col gap-3">
            <Field
              label="Khoảng thời gian poll (giây)"
              value={String(Math.round(bundle.pollIntervalMs / 1000))}
              onChange={(v) => update({ pollIntervalMs: (Number(v) || 10) * 1000 })}
            />
            <Field
              label="Timeout tạo video (giây)"
              value={String(Math.round(bundle.pollTimeoutMs / 1000))}
              onChange={(v) => update({ pollTimeoutMs: (Number(v) || 300) * 1000 })}
            />
            <div className="text-xs text-subtle">Phiên bản app: {version}</div>
          </div>
        </Card>
      </div>

      <div>
        <Button size="lg" onClick={save}>
          <Save size={16} /> Lưu cài đặt
        </Button>
      </div>
    </div>
  );
}

function Field({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="label">{label}</span>
      <input value={value ?? ''} onChange={(e) => onChange(e.target.value)} />
    </label>
  );
}

function FieldFolder({
  label, value, onChange, onBrowse,
}: { label: string; value: string; onChange: (v: string) => void; onBrowse: () => void }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="label">{label}</span>
      <div className="flex gap-2">
        <input value={value ?? ''} onChange={(e) => onChange(e.target.value)} className="flex-1 font-mono text-xs" />
        <Button variant="secondary" size="sm" onClick={onBrowse}>
          <Folder size={14} />
        </Button>
      </div>
    </label>
  );
}
