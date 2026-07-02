// ─── Risk Evaluator ───
// Determines operation risk tier and generates assessment.

import { OPERATION_RISK_MAP, OPERATION_LABELS } from '@/types/risk';
import type { RiskTier, RiskAssessment, RiskStep } from '@/types/risk';

export function evaluateRisk(
  operation: string,
  targetType: string,
  targetId: string,
  targetName: string,
): RiskAssessment {
  const tier: RiskTier = OPERATION_RISK_MAP[operation] || 'medium';

  const needsConfirmationText = [
    'delete_domain', 'delete_credential', 'delete_gateway_link',
    'uninstall_provider', 'revoke_join_token', 'revoke_api_key',
  ];

  return {
    tier,
    operation,
    target: { type: targetType, id: targetId, name: targetName },
    requiresImpact: tier !== 'low',
    requiresConfirmation: tier === 'high',
    requiresDryRun: ['apply_config', 'dry_run_apply', 'rollback'].includes(operation),
    requiresVerification: tier === 'high',
    confirmationText: needsConfirmationText.includes(operation)
      ? targetName
      : undefined,
  };
}

export function getOperationLabel(operation: string): string {
  return OPERATION_LABELS[operation] || operation;
}

export function getRiskStepOrder(tier: RiskTier): RiskStep[] {
  if (tier === 'low') return ['execute'];
  if (tier === 'medium') return ['review', 'confirm', 'execute'];
  return ['review', 'impact', 'dryrun', 'confirm', 'execute', 'verify'];
}
