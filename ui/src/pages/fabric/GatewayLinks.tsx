// ─── Gateway Links ───
// Compact rows: source → status → target. Click opens Drawer with details + actions.

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { gatewayLinkApi } from '@/lib/api-bridge';
import { PageHeader, StatusBadge, Btn, Drawer, Timestamp, Card, MetaRow, useToast } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';
import { evaluateRisk } from '@/lib/risk-evaluator';

export default function GatewayLinks() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { data } = useQuery({ queryKey: ['gateway-links'], queryFn: () => gatewayLinkApi.list() });
  const links = API_CONFIG.useMock ? getScenario().gatewayLinks : (Array.isArray(data) ? data : (data as any)?.links || []);
  const scenario = API_CONFIG.useMock ? getScenario() : null;

  const [selected, setSelected] = useState<any>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const openLink = (link: any) => {
    setSelected(link);
    setDrawerOpen(true);
  };

  // Find routes that depend on this link
  const dependentRoutes = selected && scenario
    ? scenario.routes.filter(r => r.gateway_policy?.fallback_gateway_ids?.includes(selected.gateway_link_id) || false)
    : [];

  const handleRotate = async () => {
    if (!selected) return;
    const risk = evaluateRisk('rotate_gateway_link', 'gateway_link', selected.gateway_link_id, selected.gateway_link_id);
    if (risk.tier === 'high') {
      if (!confirm(`确认轮换链路 ${selected.gateway_link_id} 的密钥？旧密钥将立即失效。`)) return;
    }
    setRotating(true);
    try {
      await gatewayLinkApi.rotate(selected.gateway_link_id);
      toast('密钥已轮换');
      queryClient.invalidateQueries({ queryKey: ['gateway-links'] });
    } catch (e: any) { toast(e.message || '轮换失败', 'error'); }
    finally { setRotating(false); }
  };

  const handleDelete = async () => {
    if (!selected) return;
    if (!confirm(`确认删除链路 ${selected.gateway_link_id}？依赖此链路的路由将无法跨节点转发。`)) return;
    setDeleting(true);
    try {
      await gatewayLinkApi.delete(selected.gateway_link_id);
      toast('链路已删除');
      queryClient.invalidateQueries({ queryKey: ['gateway-links'] });
      setDrawerOpen(false);
    } catch (e: any) { toast(e.message || '删除失败', 'error'); }
    finally { setDeleting(false); }
  };

  return (
    <div className="p-6 space-y-4">
      <PageHeader title="网关链路" subtitle={`${links.length} 条跨节点认证通道`}
        actions={<Btn primary onClick={() => toast('创建链路功能')}>创建链路</Btn>} />

      {/* Compact rows */}
      <div className="space-y-1">
        {links.map((l: any) => (
          <div key={l.gateway_link_id}
            onClick={() => openLink(l)}
            className={cn(
              'flex items-center gap-3 px-4 py-2.5 rounded-a-sm cursor-pointer transition-colors text-xs',
              'border border-a-border/40 hover:bg-a-border/10',
              l.status === 'failed' && 'bg-[#ff5c72]/3 border-l-2 border-l-[#ff5c72]',
            )}>
            <span className="font-medium text-a-fg w-28 text-right truncate">{l.source_node_name}</span>
            <div className="flex items-center gap-1.5">
              <div className={cn('w-12 h-px', l.status === 'active' ? 'bg-[#4cd964]/60' : l.status === 'failed' ? 'bg-[#ff5c72]/60' : 'bg-a-border')} />
              <StatusBadge status={l.status} />
              <div className={cn('w-12 h-px', l.status === 'active' ? 'bg-[#4cd964]/60' : l.status === 'failed' ? 'bg-[#ff5c72]/60' : 'bg-a-border')} />
            </div>
            <span className="font-medium text-a-fg w-28 truncate">{l.target_node_name}</span>
            <span className="flex-1" />
            <Timestamp iso={l.created_at} className="text-[10px]" />
          </div>
        ))}
      </div>

      {/* Detail Drawer */}
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={selected ? `${selected.source_node_name} → ${selected.target_node_name}` : '链路详情'}
        subtitle={selected?.gateway_link_id}
        width="lg"
        footer={
          <div className="flex gap-2">
            <Btn danger onClick={handleDelete} disabled={deleting}>{deleting ? '删除中...' : '删除链路'}</Btn>
            <Btn onClick={handleRotate} disabled={rotating}>{rotating ? '轮换中...' : '轮换密钥'}</Btn>
          </div>
        }
      >
        {selected && (
          <div className="space-y-4">
            <Card title="链路信息">
              <MetaRow label="状态" value={selected.status} />
              <MetaRow label="源节点" value={selected.source_node_name} />
              <MetaRow label="目标节点" value={selected.target_node_name} />
              <MetaRow label="创建时间" value={selected.created_at ? new Date(selected.created_at).toLocaleString('zh-CN') : '—'} />
              <MetaRow label="最后验证" value={selected.last_verified_at ? new Date(selected.last_verified_at).toLocaleString('zh-CN') : '未验证'} />
            </Card>

            {dependentRoutes.length > 0 && (
              <Card title={`关联路由 (${dependentRoutes.length})`} subtitle="依赖此链路的路由">
                {dependentRoutes.map((r: any) => (
                  <div key={r.route_id} className="flex items-center gap-2 text-xs py-1">
                    <span className="font-mono text-a-fg">{r.domain}</span>
                    <StatusBadge status={r.status} />
                  </div>
                ))}
              </Card>
            )}

            <div className="p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/10 text-[11px] text-a-muted">
              ⚠️ 轮换密钥将使旧密钥立即失效。依赖此链路的所有跨节点转发将短暂中断直到配置重新下发。
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
