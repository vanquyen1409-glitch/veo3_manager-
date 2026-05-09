import { CheckCircle2, AlertTriangle, AlertCircle, Info, X } from 'lucide-react';
import { useAppStore, type ToastKind } from '../store/appStore';
import { cn } from '../lib/cn';

const styles: Record<ToastKind, { icon: typeof Info; iconCls: string; cls: string }> = {
  info:    { icon: Info,          iconCls: 'text-slate-500 dark:text-slate-300',  cls: 'surface-panel divider-strong' },
  success: { icon: CheckCircle2,  iconCls: 'text-emerald-600 dark:text-emerald-400', cls: 'surface-panel border-emerald-400 dark:border-emerald-700' },
  warn:    { icon: AlertTriangle, iconCls: 'text-amber-600 dark:text-amber-400',  cls: 'surface-panel border-amber-400 dark:border-amber-700' },
  error:   { icon: AlertCircle,   iconCls: 'text-red-600 dark:text-red-400',      cls: 'surface-panel border-red-400 dark:border-red-700' },
};

export default function Toaster() {
  const toasts = useAppStore((s) => s.toasts);
  const dismiss = useAppStore((s) => s.dismissToast);

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2">
      {toasts.map((t) => {
        const { icon: Icon, iconCls, cls } = styles[t.kind];
        return (
          <div
            key={t.id}
            className={cn(
              'pointer-events-auto rounded-lg border p-3 shadow-lg',
              cls,
            )}
          >
            <div className="flex items-start gap-2">
              <Icon size={16} className={cn('mt-0.5 flex-none', iconCls)} />
              <div className="flex-1 text-sm">
                <div className="text-strong font-medium">{t.title}</div>
                {t.body && <div className="text-muted mt-0.5 text-xs">{t.body}</div>}
              </div>
              <button
                onClick={() => dismiss(t.id)}
                className="text-subtle hover-surface hover:text-strong rounded p-0.5"
                aria-label="Dismiss"
              >
                <X size={14} />
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
