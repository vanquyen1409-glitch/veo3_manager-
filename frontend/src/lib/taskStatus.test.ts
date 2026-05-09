import { describe, it, expect } from 'vitest';
import { taskPhaseLabel, taskPhaseTone, taskStatusBadge } from './taskStatus';

describe('taskPhaseLabel', () => {
  it('maps each known phase to a Vietnamese label', () => {
    expect(taskPhaseLabel('pending')).toBe('chờ xử lý');
    expect(taskPhaseLabel('running')).toBe('đang chạy');
    expect(taskPhaseLabel('submitted')).toBe('đã gửi');
    expect(taskPhaseLabel('polling')).toBe('đang chờ');
    expect(taskPhaseLabel('downloading')).toBe('đang tải');
    expect(taskPhaseLabel('succeeded')).toBe('thành công');
    expect(taskPhaseLabel('failed')).toBe('thất bại');
    expect(taskPhaseLabel('cancelled')).toBe('đã hủy');
  });

  it('falls back to the raw phase string for unknown values', () => {
    expect(taskPhaseLabel('something_new')).toBe('something_new');
  });

  it('returns "" for undefined to avoid "undefined" leaking into UI', () => {
    expect(taskPhaseLabel(undefined)).toBe('');
  });
});

describe('taskPhaseTone', () => {
  it('returns a non-empty class string for every known phase', () => {
    for (const phase of ['pending', 'running', 'succeeded', 'failed', 'cancelled', 'submitted', 'polling', 'downloading']) {
      const cls = taskPhaseTone(phase);
      expect(cls.length).toBeGreaterThan(0);
      expect(cls).toMatch(/text-/);
    }
  });

  it('falls back to grey for unknown phase', () => {
    expect(taskPhaseTone('mystery')).toBe('text-slate-400');
  });
});

describe('taskStatusBadge', () => {
  it.each([
    'succeeded', 'failed', 'cancelled', 'running', 'pending',
  ] as const)('returns dark+light variant classes for %s', (status) => {
    const cls = taskStatusBadge(status);
    expect(cls).toMatch(/bg-/);
    expect(cls).toMatch(/text-/);
    expect(cls).toMatch(/border/);
    expect(cls).toMatch(/dark:/);
  });

  it('returns the default-grey badge for unknown', () => {
    expect(taskStatusBadge('mystery')).toMatch(/text-slate-600/);
  });

  it('handles undefined gracefully', () => {
    expect(taskStatusBadge(undefined)).toMatch(/text-slate-600/);
  });
});
