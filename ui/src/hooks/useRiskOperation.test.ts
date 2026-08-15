import { describe, expect, it } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useRiskOperation } from '@/hooks/useRiskOperation';

// Regression test: execute() only advanced the step for LOW tier, so the
// high-risk apply wizard never showed its execute/verify steps after a
// successful push — the user was bounced back to the confirm step.
describe('useRiskOperation execute step advancement', () => {
  it('high tier advances to verify after success', async () => {
    const { result } = renderHook(() =>
      useRiskOperation('apply_config', 'config', 'current', '当前配置'),
    );
    expect(result.current.tier).toBe('high');

    await act(async () => {
      await result.current.execute(async () => { /* simulated push */ });
    });

    await waitFor(() => {
      expect(result.current.step).toBe('verify');
    });
    expect(result.current.executing).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('low tier advances to done after success', async () => {
    const { result } = renderHook(() =>
      useRiskOperation('run_health_check', 'health', 'svc-1', 'svc-1'),
    );
    expect(result.current.tier).toBe('low');

    await act(async () => {
      await result.current.execute(async () => {});
    });

    await waitFor(() => {
      expect(result.current.step).toBe('done');
    });
  });

  it('moves to error and rethrows on failure', async () => {
    const { result } = renderHook(() =>
      useRiskOperation('apply_config', 'config', 'current', '当前配置'),
    );

    let caught: unknown = null;
    await act(async () => {
      try {
        await result.current.execute(async () => { throw new Error('boom'); });
      } catch (e) {
        caught = e;
      }
    });

    expect(caught).toBeInstanceOf(Error);
    expect(result.current.step).toBe('error');
    expect(result.current.error).toBe('boom');
    expect(result.current.executing).toBe(false);
  });
});
