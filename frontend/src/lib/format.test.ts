import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatDate, formatRelative, formatDuration, truncate } from './format';

describe('formatDate', () => {
  it('returns em-dash for null/undefined/empty', () => {
    expect(formatDate(null)).toBe('—');
    expect(formatDate(undefined)).toBe('—');
    expect(formatDate('')).toBe('—');
  });

  it('returns em-dash for invalid dates rather than "Invalid Date"', () => {
    expect(formatDate('not a date')).toBe('—');
    expect(formatDate(new Date('garbage'))).toBe('—');
  });

  it('formats valid Date and ISO string equivalently', () => {
    const d = new Date('2026-05-09T12:00:00Z');
    expect(formatDate(d)).toBe(formatDate('2026-05-09T12:00:00Z'));
  });
});

describe('formatRelative', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-09T12:00:00Z'));
  });
  afterEach(() => vi.useRealTimers());

  it('< 60s shows seconds', () => {
    expect(formatRelative(new Date('2026-05-09T11:59:30Z'))).toMatch(/^\d+s ago$/);
  });

  it('< 60min shows minutes', () => {
    expect(formatRelative(new Date('2026-05-09T11:30:00Z'))).toBe('30m ago');
  });

  it('< 24h shows hours', () => {
    expect(formatRelative(new Date('2026-05-09T08:00:00Z'))).toBe('4h ago');
  });

  it('>= 24h shows days', () => {
    expect(formatRelative(new Date('2026-05-04T12:00:00Z'))).toBe('5d ago');
  });

  it('returns em-dash for falsy', () => {
    expect(formatRelative(null)).toBe('—');
    expect(formatRelative(undefined)).toBe('—');
  });
});

describe('formatDuration', () => {
  it('returns em-dash for 0/negative/falsy', () => {
    expect(formatDuration(0)).toBe('—');
    expect(formatDuration(-100)).toBe('—');
    expect(formatDuration(NaN)).toBe('—');
  });

  it('< 1 minute shows seconds only', () => {
    expect(formatDuration(45_000)).toBe('45s');
  });

  it('>= 1 minute shows m s', () => {
    expect(formatDuration(2 * 60_000 + 30_000)).toBe('2m 30s');
  });

  it('exactly 60s shows 1m 0s, not 60s', () => {
    expect(formatDuration(60_000)).toBe('1m 0s');
  });
});

describe('truncate', () => {
  it('returns "" for empty/falsy', () => {
    expect(truncate('')).toBe('');
    // @ts-expect-error null guard
    expect(truncate(null)).toBe('');
  });

  it('returns input unchanged when shorter than n', () => {
    expect(truncate('hi', 80)).toBe('hi');
  });

  it('truncates with ellipsis at exactly n chars total', () => {
    const r = truncate('a'.repeat(100), 10);
    expect(r.length).toBe(10);
    expect(r.endsWith('…')).toBe(true);
  });

  it('default n is 80', () => {
    const r = truncate('a'.repeat(200));
    expect(r.length).toBe(80);
  });
});
