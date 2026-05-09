// Vitest setup: extends `expect` with @testing-library/jest-dom matchers
// (toBeInTheDocument, toHaveClass, etc.) so component tests read naturally.
import '@testing-library/jest-dom';

// Stub the auto-generated Wails bindings imported by pages/features. The real
// runtime is only available inside the desktop shell; in unit tests every
// hook just resolves to the test fixture.
import { vi } from 'vitest';

vi.mock('../../wailsjs/go/main/App', () => ({
  ListTasks: vi.fn(async () => []),
  Enqueue: vi.fn(async () => ({ id: 'fake', prompt: '', status: 'pending' })),
  DeleteTask: vi.fn(async () => undefined),
  RequeueTask: vi.fn(async () => ({ id: 'new', status: 'pending' })),
  Stats: vi.fn(async () => ({ today: { total: 0, succeeded: 0, failed: 0 } })),
  GetSettings: vi.fn(async () => ({})),
  SetSettings: vi.fn(async () => undefined),
}));

vi.mock('../../wailsjs/go/models', () => ({}));

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(() => () => undefined),
  EventsOff: vi.fn(),
  EventsEmit: vi.fn(),
}));
