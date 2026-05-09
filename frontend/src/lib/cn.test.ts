import { describe, it, expect } from 'vitest';
import { cn } from './cn';

describe('cn (clsx + tailwind-merge)', () => {
  it('joins multiple class names', () => {
    expect(cn('a', 'b', 'c')).toBe('a b c');
  });

  it('drops falsy values', () => {
    expect(cn('a', false, undefined, null, 0, 'b')).toBe('a b');
  });

  it('dedupes conflicting tailwind classes (later wins)', () => {
    // Classic twMerge behaviour - p-2 should be wiped by p-4 because they
    // map to the same Tailwind cluster.
    expect(cn('p-2', 'p-4')).toBe('p-4');
  });

  it('preserves non-conflicting classes', () => {
    expect(cn('p-2', 'm-4')).toBe('p-2 m-4');
  });

  it('handles object form', () => {
    expect(cn({ active: true, disabled: false })).toBe('active');
  });
});
