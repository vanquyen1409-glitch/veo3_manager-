import type { CSSProperties } from 'react';
import { Minus, Square, X } from 'lucide-react';
import {
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
} from '../../wailsjs/runtime/runtime';
import ThemeToggle from './ThemeToggle';

const dragRegion: CSSProperties = {
  // @ts-expect-error custom Wails CSS prop
  '--wails-draggable': 'drag',
};

const noDrag: CSSProperties = {
  // @ts-expect-error custom Wails CSS prop
  '--wails-draggable': 'no-drag',
};

export default function TitleBar() {
  return (
    <header
      style={dragRegion}
      className="surface-app flex h-9 select-none items-center justify-between border-b divider pl-4 text-xs"
    >
      <span className="text-strong font-medium tracking-wide">Veo3 Manager</span>
      <div style={noDrag} className="flex h-full items-center gap-1 pr-1">
        <ThemeToggle />
        <button type="button" onClick={() => WindowMinimise()} className="text-muted hover-surface hover:text-strong flex h-full w-11 items-center justify-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent-500" aria-label="Thu nhỏ">
          <Minus size={14} />
        </button>
        <button type="button" onClick={() => WindowToggleMaximise()} className="text-muted hover-surface hover:text-strong flex h-full w-11 items-center justify-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent-500" aria-label="Phóng to">
          <Square size={12} />
        </button>
        <button type="button" onClick={() => Quit()} className="text-muted flex h-full w-11 items-center justify-center hover:bg-danger-600 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-danger-500" aria-label="Đóng">
          <X size={14} />
        </button>
      </div>
    </header>
  );
}
