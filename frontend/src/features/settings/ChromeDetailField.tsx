import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';

export function truncateMid(s: string, max = 96): string {
  if (!s || s.length <= max) return s ?? '';
  const head = Math.floor((max - 3) / 2);
  const tail = max - 3 - head;
  return `${s.slice(0, head)}…${s.slice(-tail)}`;
}

export function Field({
  label,
  value,
  mono,
  actions,
}: {
  label: ReactNode;
  value: ReactNode;
  mono?: boolean;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="label">{label}</span>
      <div className="flex items-center gap-2">
        <div
          className={cn(
            'surface-elev text-body flex-1 break-all rounded-md border divider px-3 py-2 text-xs',
            mono && 'font-mono',
          )}
        >
          {value}
        </div>
        {actions}
      </div>
    </div>
  );
}
