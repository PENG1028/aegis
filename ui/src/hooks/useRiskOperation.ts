// ─── useRiskOperation Hook ───
// Core hook encapsulating the full risk-tiered operation lifecycle.
// Low risk: execute immediately
// Medium risk: Drawer → ImpactPanel → Confirm → Execute
// High risk: Wizard → Review → Impact → DryRun → Confirm → Execute → Verify

import { useState, useCallback } from 'react';
import { evaluateRisk, getRiskStepOrder } from '@/lib/risk-evaluator';
import type { RiskTier, RiskStep, RiskAssessment } from '@/types/risk';
import type { ImpactScope } from '@/types/impact';

export interface RiskOperationState {
  assessment: RiskAssessment;
  step: RiskStep;
  impact: ImpactScope | null;
  dryRunResult: unknown | null;
  verifyResult: unknown | null;
  error: string | null;
  executing: boolean;
}

export function useRiskOperation(
  operation: string,
  targetType: string,
  targetId: string,
  targetName: string,
) {
  const assessment = evaluateRisk(operation, targetType, targetId, targetName);
  const steps = getRiskStepOrder(assessment.tier);

  const [state, setState] = useState<RiskOperationState>({
    assessment,
    step: steps[0],
    impact: null,
    dryRunResult: null,
    verifyResult: null,
    error: null,
    executing: false,
  });

  const advanceTo = useCallback((nextStep: RiskStep) => {
    setState(s => ({ ...s, step: nextStep }));
  }, []);

  const setImpact = useCallback((impact: ImpactScope) => {
    setState(s => ({ ...s, impact }));
  }, []);

  const setDryRunResult = useCallback((result: unknown) => {
    setState(s => ({ ...s, dryRunResult: result }));
  }, []);

  const setVerifyResult = useCallback((result: unknown) => {
    setState(s => ({ ...s, verifyResult: result }));
  }, []);

  const setError = useCallback((error: string | null) => {
    setState(s => ({ ...s, error, executing: false }));
  }, []);

  const execute = useCallback(async (fn: () => Promise<void>) => {
    setState(s => ({ ...s, executing: true, error: null }));
    try {
      await fn();
      if (assessment.tier === 'low') {
        setState(s => ({ ...s, step: 'done', executing: false }));
      }
    } catch (e) {
      setState(s => ({
        ...s,
        step: 'error',
        error: e instanceof Error ? e.message : '操作失败',
        executing: false,
      }));
      throw e;
    }
  }, [assessment.tier]);

  const reset = useCallback(() => {
    setState({
      assessment,
      step: steps[0],
      impact: null,
      dryRunResult: null,
      verifyResult: null,
      error: null,
      executing: false,
    });
  }, [assessment, steps]);

  return {
    ...state,
    tier: assessment.tier,
    steps,
    advanceTo,
    setImpact,
    setDryRunResult,
    setVerifyResult,
    setError,
    execute,
    reset,
  };
}
