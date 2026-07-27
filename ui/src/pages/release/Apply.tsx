// ─── Apply Config ───
// High-risk Wizard flow: Review → Impact → DryRun → Confirm → Execute → Verify
import { useState } from 'react';
import { useRiskOperation } from '@/hooks/useRiskOperation';
import { PageHeader, Card, Wizard, Btn, useToast } from '@/components/shared';
import { ImpactPanel } from '@/components/workspace/ImpactPanel';
import { ReleaseDiffViewer } from '@/components/workspace/ReleaseDiffViewer';
import { useDiff } from '@/hooks/useDiff';
import { adminApi } from '@/lib/api-bridge';
import type { WizardStep } from '@/components/shared/Wizard';
import type { ImpactScope } from '@/types/impact';

const APPLY_STEPS: WizardStep[] = [
  { key: 'review', title: '审核', description: '查看变更内容' },
  { key: 'impact', title: '影响', description: '分析影响范围' },
  { key: 'dryrun', title: '预演', description: 'Dry-run 模拟' },
  { key: 'confirm', title: '确认', description: '输入确认文字' },
  { key: 'execute', title: '执行', description: '推送配置' },
  { key: 'verify', title: '验证', description: '验证结果' },
];

export default function Apply() {
  const toast = useToast();
  const { data: diff, isLoading: diffLoading } = useDiff();
  const [confirmText, setConfirmText] = useState('');
  const [wizardOpen, setWizardOpen] = useState(false);

  const op = useRiskOperation('apply_config', 'config', 'current', '当前配置');

  const mockImpact: ImpactScope = {
    target: { type: 'config', id: 'current', name: '当前配置' },
    operation: 'apply_config',
    affectedEntries: [{ type: 'route', id: 'route-docs', name: 'docs.proofnote.dev', status: 'healthy', impact: 'direct', description: '新增路由将生效' }],
    affectedServices: [{ type: 'service', id: 'service-docs', name: 'docs-service', status: 'healthy', impact: 'direct', description: '服务配置将更新' }],
    affectedGateways: [],
    affectedNodes: [{ type: 'node', id: 'node-c', name: 'Server C', status: 'online', impact: 'indirect', description: '新端点将部署到此节点' }],
    totalAffected: 3,
    hasDownstreamFailures: false,
  };

  const handleApply = async () => {
    try {
      await op.execute(async () => {
        await adminApi.applyConfig();
        toast('配置推送成功');
      });
    } catch {
      // Error handled by hook
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="推送配置" subtitle="高风险操作 · 审核 → 影响分析 → Dry-run → 确认 → 执行 → 验证" />
      <Card title="发布中心">
        <div className="text-center py-6 space-y-4">
          <p className="text-sm text-a-fg2">当前有待发布的配置变更</p>
          <Btn danger onClick={() => setWizardOpen(true)}>开始推送配置</Btn>
        </div>
      </Card>

      <Wizard
        open={wizardOpen}
        steps={APPLY_STEPS}
        currentStep={op.step}
        onClose={() => { setWizardOpen(false); op.reset(); }}
        footer={
          <div className="flex gap-2">
            {op.step === 'review' && <Btn onClick={() => op.advanceTo('impact')}>下一步：影响分析</Btn>}
            {op.step === 'impact' && <><Btn onClick={() => op.advanceTo('review')}>上一步</Btn><Btn primary onClick={() => op.advanceTo('dryrun')}>下一步：Dry-run</Btn></>}
            {op.step === 'dryrun' && <><Btn onClick={() => op.advanceTo('impact')}>上一步</Btn><Btn primary onClick={() => op.advanceTo('confirm')}>下一步：确认</Btn></>}
            {op.step === 'confirm' && <><Btn onClick={() => op.advanceTo('dryrun')}>上一步</Btn>
              <input className="px-3 py-1.5 text-xs rounded-a-md bg-a-bg border border-a-border text-a-fg" placeholder='输入 "APPLY" 确认' value={confirmText} onChange={e => setConfirmText(e.target.value)} />
              <Btn danger disabled={confirmText !== 'APPLY' || op.executing} onClick={handleApply}>{op.executing ? '推送中...' : '确认推送'}</Btn></>}
            {op.step === 'execute' && <Btn disabled className="text-xs">正在执行...</Btn>}
            {op.step === 'verify' && <Btn onClick={() => { setWizardOpen(false); op.reset(); }}>完成</Btn>}
            {op.step === 'error' && <><Btn onClick={() => op.reset()}>重试</Btn><Btn onClick={() => setWizardOpen(false)}>取消</Btn></>}
          </div>
        }
      >
        {op.step === 'review' && <ReleaseDiffViewer diff={diff || null} loading={diffLoading} />}
        {op.step === 'impact' && <ImpactPanel impact={mockImpact} />}
        {op.step === 'dryrun' && <div className="text-center py-6"><p className="text-sm text-a-muted">Dry-run 结果将在此显示</p><Btn primary className="mt-3" onClick={() => adminApi.dryRun().then(() => toast('Dry-run 通过')).catch(e => toast(e.message, 'error'))}>执行 Dry-run</Btn></div>}
        {op.step === 'confirm' && <div className="text-center py-6 space-y-3"><div className="text-4xl">⚠️</div><p className="text-sm text-a-fg2">此操作将修改生产配置，影响正在运行的网关</p><p className="text-xs text-[#ff5c72]">请输入 APPLY 确认推送</p></div>}
        {op.step === 'execute' && <div className="text-center py-6"><div className="text-3xl mb-3">🔄</div><p className="text-sm text-a-muted">正在推送配置到所有节点...</p></div>}
        {op.step === 'verify' && <div className="text-center py-6"><div className="text-3xl mb-3">✅</div><p className="text-sm text-[#4cd964]">配置推送成功，所有节点已同步</p></div>}
        {op.step === 'error' && <div className="text-center py-6"><div className="text-3xl mb-3">❌</div><p className="text-sm text-[#ff5c72]">{op.error || '操作失败'}</p></div>}
      </Wizard>
    </div>
  );
}
