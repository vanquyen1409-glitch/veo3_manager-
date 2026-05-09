import { useEffect, useRef } from 'react';
import { AlertTriangle } from 'lucide-react';
import { useAppStore } from '../store/appStore';
import { Button } from './ui/Button';

/**
 * Single in-app confirm dialog driven by appStore.confirm. Replaces
 * window.confirm() so the prompt matches the dark theme, supports keyboard
 * navigation (Esc cancels, Enter confirms), and traps focus in the modal.
 */
export default function ConfirmDialog() {
  const req = useAppStore((s) => s.confirm);
  const resolve = useAppStore((s) => s.resolveConfirm);
  const confirmBtnRef = useRef<HTMLButtonElement>(null);
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!req) return;
    confirmBtnRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault();
        resolve();
        return;
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        runConfirm();
        return;
      }
      if (e.key === 'Tab') {
        // Trap focus inside the dialog.
        const focusables = dialogRef.current?.querySelectorAll<HTMLElement>(
          'button, [href], input, [tabindex]:not([tabindex="-1"])',
        );
        if (!focusables || focusables.length === 0) return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
          return;
        }
        if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [req, resolve]);

  if (!req) return null;

  async function runConfirm() {
    if (!req) return;
    try {
      await req.onConfirm();
    } finally {
      resolve();
    }
  }

  const danger = req.danger ?? false;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      aria-describedby={req.body ? 'confirm-body' : undefined}
      className="fixed inset-0 z-[60] flex items-center justify-center p-4"
    >
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={resolve}
        aria-hidden="true"
      />
      <div
        ref={dialogRef}
        className="surface-panel relative w-full max-w-md rounded-xl p-5 shadow-2xl"
      >
        <div className="flex items-start gap-3">
          {danger && (
            <span className="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-red-500/15">
              <AlertTriangle size={18} className="text-red-500 dark:text-red-400" />
            </span>
          )}
          <div className="flex-1">
            <h2 id="confirm-title" className="text-strong text-base font-semibold">
              {req.title}
            </h2>
            {req.body && (
              <p id="confirm-body" className="mt-1.5 text-sm text-muted">
                {req.body}
              </p>
            )}
          </div>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" size="md" onClick={resolve}>
            {req.cancelLabel ?? 'Hủy'}
          </Button>
          <Button
            ref={confirmBtnRef}
            variant={danger ? 'danger' : 'primary'}
            size="md"
            onClick={runConfirm}
          >
            {req.confirmLabel ?? 'Xác nhận'}
          </Button>
        </div>
      </div>
    </div>
  );
}
