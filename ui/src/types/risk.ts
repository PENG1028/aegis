// ─── Risk-Tiered Operation Types ───

export type RiskTier = 'low' | 'medium' | 'high';

export interface RiskAssessment {
  tier: RiskTier;
  operation: string;
  target: { type: string; id: string; name: string };
  requiresImpact: boolean;
  requiresConfirmation: boolean;
  requiresDryRun: boolean;
  requiresVerification: boolean;
  confirmationText?: string;
}

/** Maps operation names to their risk tier. */
export const OPERATION_RISK_MAP: Record<string, RiskTier> = {
  // Low risk — execute directly
  refresh_health: 'low',
  run_health_check: 'low',
  diagnose: 'low',
  refresh_capabilities: 'low',
  preview_config: 'low',
  view_diff: 'low',
  // Medium risk — Drawer with ImpactPanel
  disable_endpoint: 'medium',
  enable_endpoint: 'medium',
  reload_provider: 'medium',
  trigger_node_update: 'medium',
  refresh_dns: 'medium',
  activate_exposure: 'medium',
  disable_exposure: 'medium',
  // High risk — Wizard: Review → Impact → DryRun → Confirm → Execute → Verify
  apply_config: 'high',
  dry_run_apply: 'high',
  rollback: 'high',
  delete_domain: 'high',
  reveal_credential: 'high',
  delete_credential: 'high',
  rotate_credential: 'high',
  rotate_api_key: 'high',
  rotate_gateway_link: 'high',
  disable_route: 'high',
  enable_route: 'high',
  uninstall_provider: 'high',
  install_provider: 'high',
  delete_gateway_link: 'high',
  revoke_join_token: 'high',
  revoke_api_key: 'high',
  upload_binary: 'high',
  deploy_node: 'high',
};

/** User-friendly labels for operations. */
export const OPERATION_LABELS: Record<string, string> = {
  refresh_health: '刷新健康检查',
  run_health_check: '运行健康检查',
  diagnose: '诊断',
  refresh_capabilities: '刷新节点能力',
  preview_config: '预览配置',
  view_diff: '查看差异',
  disable_endpoint: '禁用端点',
  enable_endpoint: '启用端点',
  reload_provider: '重载 Provider',
  trigger_node_update: '触发节点更新',
  refresh_dns: '刷新 DNS',
  activate_exposure: '激活端口暴露',
  disable_exposure: '停用端口暴露',
  apply_config: '推送配置',
  dry_run_apply: 'Dry-run 预览',
  rollback: '回滚配置',
  delete_domain: '删除域名',
  reveal_credential: '揭示凭据',
  delete_credential: '删除凭据',
  rotate_credential: '轮换凭据',
  rotate_api_key: '轮换 API 密钥',
  rotate_gateway_link: '轮换链路密钥',
  disable_route: '禁用路由',
  enable_route: '启用路由',
  uninstall_provider: '卸载 Provider',
  install_provider: '安装 Provider',
  delete_gateway_link: '删除网关链路',
  revoke_join_token: '撤销加入令牌',
  revoke_api_key: '撤销 API 密钥',
  upload_binary: '上传二进制',
  deploy_node: '部署节点',
};

export type RiskStep =
  | 'idle'
  | 'review'
  | 'impact'
  | 'dryrun'
  | 'confirm'
  | 'execute'
  | 'verify'
  | 'done'
  | 'error';
