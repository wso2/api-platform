import { describe, expect, it } from 'vitest';

import { relativeTime } from './relativeTime';

describe('relativeTime', () => {
  it('returns empty string for missing or invalid input', () => {
    expect(relativeTime(undefined)).toBe('');
    expect(relativeTime('')).toBe('');
    expect(relativeTime('not-a-date')).toBe('');
  });

  it('formats a recent past time in hours', () => {
    const threeHoursAgo = new Date(Date.now() - 3 * 60 * 60 * 1000);
    expect(relativeTime(threeHoursAgo)).toBe('3 hours ago');
  });

  it('formats days for older timestamps', () => {
    const twoDaysAgo = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000);
    expect(relativeTime(twoDaysAgo)).toBe('2 days ago');
  });
});
