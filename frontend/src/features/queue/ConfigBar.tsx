import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import type { db } from '../../../wailsjs/go/models';
import { cn } from '../../lib/cn';

// Aspect ratios available on Flow's Video tab. Image tab has more (4:3, 3:4,
// 1:1) but Veo only supports landscape + portrait at the moment.
const ratios: { label: string; value: string }[] = [
  { label: '16:9', value: '16:9' }, // VIDEO_ASPECT_RATIO_LANDSCAPE
  { label: '9:16', value: '9:16' }, // VIDEO_ASPECT_RATIO_PORTRAIT
];

const counts = [1, 2, 3, 4];

// Map raw API model keys → human-friendly labels. Keep keys in sync with
// internal/labsapi/constants.go (DefaultModel) and internal/db/settings.go.
const modelLabels: Record<string, string> = {
  veo_3_1_t2v_fast: 'Veo 3.1 Fast',
};

interface Props {
  cfg: db.GenerationConfig;
  onChange: (c: db.GenerationConfig) => void;
}

export default function ConfigBar({ cfg, onChange }: Props) {
  return (
    <Card>
      <CardHeader><CardTitle>Cấu hình tạo video</CardTitle></CardHeader>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div>
          <div className="label mb-2">Tỷ lệ khung hình</div>
          <div className="surface-elev inline-flex rounded-md border divider p-0.5 text-sm">
            {ratios.map((r) => (
              <button
                key={r.value}
                onClick={() => onChange({ ...cfg, aspectRatio: r.value })}
                className={cn(
                  'rounded px-3 py-1.5 transition',
                  cfg.aspectRatio === r.value ? 'bg-accent-600 text-white' : 'text-soft hover-surface',
                )}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <div className="label mb-2">Số video / prompt</div>
          <div className="surface-elev inline-flex rounded-md border divider p-0.5 text-sm">
            {counts.map((c) => (
              <button
                key={c}
                onClick={() => onChange({ ...cfg, outputCount: c })}
                className={cn(
                  'w-10 rounded py-1.5 transition',
                  cfg.outputCount === c ? 'bg-accent-600 text-white' : 'text-soft hover-surface',
                )}
              >
                {c}
              </button>
            ))}
          </div>
        </div>

        <div>
          <div className="label mb-2">Model</div>
          <div className="surface-elev text-body inline-flex h-9 items-center rounded-md border divider px-3 text-sm">
            {modelLabels[cfg.model] ?? cfg.model ?? '—'}
          </div>
        </div>
      </div>
    </Card>
  );
}
