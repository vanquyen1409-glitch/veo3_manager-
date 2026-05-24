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

// Model FAMILIES exposed by labs.google Flow. The `key` here is the family id
// stored in GenerationConfig.model — the backend (labsapi.VideoModelKeyFor)
// resolves it to the real per-aspect videoModelKey at submit time. All four
// keys + credit costs were confirmed from the live Flow flow.projectInitialData
// response (2026-05-24); Fast's derived key matched the app's original
// known-working id, validating the rest.
//
// `credits` are for the user's SERVICE_TIER_INTERMEDIATE tier (matches the
// "N tín dụng" shown in the Flow UI). Omni Flash varies by clip length, so its
// per-duration costs live in `omniCredits` and the flat field is null.
type ModelOption = {
  key: string;
  label: string;
  credits: number | null;
};

const models: ModelOption[] = [
  { key: 'veo_3_1_lite', label: 'Lite', credits: 10 },
  { key: 'veo_3_1_fast', label: 'Fast', credits: 20 },
  { key: 'veo_3_1_quality', label: 'Quality', credits: 100 },
  { key: 'abra', label: 'Omni Flash', credits: null },
];

// Omni Flash (abra) clip lengths and their INTERMEDIATE-tier credit cost.
const OMNI_FAMILY = 'abra';
const omniDurations: { sec: number; credits: number }[] = [
  { sec: 4, credits: 15 },
  { sec: 6, credits: 20 },
  { sec: 8, credits: 25 },
  { sec: 10, credits: 30 },
];

// Credits shown for a model chip — Omni's depends on the chosen clip length.
function modelCredits(m: ModelOption, omniSec: number): number {
  if (m.key === OMNI_FAMILY) {
    return omniDurations.find((d) => d.sec === omniSec)?.credits ?? 25;
  }
  return m.credits ?? 0;
}

interface Props {
  cfg: db.GenerationConfig;
  onChange: (c: db.GenerationConfig) => void;
}

export default function ConfigBar({ cfg, onChange }: Props) {
  return (
    <Card>
      <CardHeader><CardTitle>Cấu hình tạo video</CardTitle></CardHeader>

      <div className="flex flex-wrap gap-x-8 gap-y-4">
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
          <div className="label mb-2">Model (chất lượng)</div>
          <div className="surface-elev inline-flex rounded-md border divider p-0.5 text-sm">
            {models.map((m) => {
              const selected = cfg.model === m.key;
              const credits = modelCredits(m, cfg.omniDurationSec);
              return (
                <button
                  key={m.key}
                  onClick={() => onChange({ ...cfg, model: m.key })}
                  title={`${m.label} • ${credits} tín dụng / video`}
                  className={cn(
                    'flex items-center gap-1.5 rounded px-3 py-1.5 transition',
                    selected ? 'bg-accent-600 text-white' : 'text-soft hover-surface',
                  )}
                >
                  <span>{m.label}</span>
                  <span className={cn(
                    'text-[10px]',
                    selected ? 'text-white/80' : 'text-subtle',
                  )}>
                    {credits}đ
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        {/* Omni Flash is the only family with a selectable clip length. */}
        {cfg.model === OMNI_FAMILY && (
          <div>
            <div className="label mb-2">Thời lượng (Omni)</div>
            <div className="surface-elev inline-flex rounded-md border divider p-0.5 text-sm">
              {omniDurations.map((d) => (
                <button
                  key={d.sec}
                  onClick={() => onChange({ ...cfg, omniDurationSec: d.sec })}
                  title={`${d.sec}s • ${d.credits} tín dụng / video`}
                  className={cn(
                    'rounded px-3 py-1.5 transition',
                    cfg.omniDurationSec === d.sec ? 'bg-accent-600 text-white' : 'text-soft hover-surface',
                  )}
                >
                  {d.sec}s
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}
