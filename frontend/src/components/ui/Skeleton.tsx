import { cn } from '../../lib/cn';

interface Props {
  className?: string;
}

/**
 * Pulsing placeholder block. Compose multiple of these to build skeleton
 * layouts that match the real content's shape.
 */
export function Skeleton({ className }: Props) {
  return (
    <div
      className={cn(
        'animate-pulse rounded-md bg-slate-200/70 dark:bg-slate-800/70',
        className,
      )}
      aria-hidden="true"
    />
  );
}

/**
 * Pre-composed row skeleton matching the History/Queue list shape.
 */
export function SkeletonRow() {
  return (
    <div className="flex items-center gap-3 px-4 py-2.5">
      <Skeleton className="h-3 w-32 flex-none" />
      <Skeleton className="h-3 flex-1" />
      <Skeleton className="h-4 w-20 flex-none" />
      <Skeleton className="h-3 w-14 flex-none" />
      <Skeleton className="h-3 w-6 flex-none" />
    </div>
  );
}

/**
 * Pre-composed card skeleton for settings / detail panels.
 */
export function SkeletonCard() {
  return (
    <div className="surface-panel flex flex-col gap-3 rounded-lg p-4">
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-5/6" />
      <Skeleton className="h-3 w-4/6" />
    </div>
  );
}
