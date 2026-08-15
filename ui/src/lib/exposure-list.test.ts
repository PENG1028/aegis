import { describe, expect, it } from 'vitest';
import { normalizeExposureList } from '@/lib/exposure-list';

describe('normalizeExposureList', () => {
  it('accepts a bare array (current backend shape)', () => {
    const arr = [{ id: 'exp_1', type: 'tcp' }];
    expect(normalizeExposureList(arr)).toEqual(arr);
  });

  it('accepts { data: [...] }', () => {
    const arr = [{ id: 'exp_2' }];
    expect(normalizeExposureList({ data: arr, meta: { total: 1 } })).toEqual(arr);
  });

  it('accepts { exposures: [...] } (legacy client contract)', () => {
    const arr = [{ id: 'exp_3' }];
    expect(normalizeExposureList({ exposures: arr, count: 1 })).toEqual(arr);
  });

  it('returns [] for null/undefined/malformed', () => {
    expect(normalizeExposureList(null)).toEqual([]);
    expect(normalizeExposureList(undefined)).toEqual([]);
    expect(normalizeExposureList({ routes: [] })).toEqual([]);
    expect(normalizeExposureList('nope')).toEqual([]);
  });
});
