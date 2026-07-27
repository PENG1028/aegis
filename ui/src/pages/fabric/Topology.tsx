// ─── Topology ───
// Compact table: 节点间连通性矩阵。Click opens Drawer with edge details.

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchTopologyMatrix } from '@/lib/api-bridge';
import { PageHeader, StatusBadge, Drawer, Card, MetaRow, Btn, useToast } from '@/components/shared';

import { cn } from '@/lib/utils';

export default function Topology() {
  const toast = useToast();
  const { data } = useQuery({ queryKey: ['topology'], queryFn: fetchTopologyMatrix });
  
  const edges = (data || []);

  const [selected, setSelected] = useState<any>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const openEdge = (e: any) => {
    setSelected(e);
    setDrawerOpen(true);
  };

  // Find the linked GatewayLink if any
  const linkedLink: any = null;

  return (
    <div className="p-6 space-y-4">
      <PageHeader title="网络拓扑" subtitle={`${edges.length} 条节点间链路 · 连通性矩阵`} />

      {/* Column headers */}
      <div className="flex items-center gap-3 px-4 py-1.5 text-[10px] text-a-muted uppercase tracking-wider border-b border-a-border/30">
        <span className="w-28 text-right">源节点</span>
        <span className="w-20 text-center" />
        <span className="w-28">目标节点</span>
        <span className="w-14 text-center">私网</span>
        <span className="w-14 text-center">公网</span>
        <span className="flex-1">状态 / 关联 Link</span>
        <span className="w-40 text-right">最后错误</span>
      </div>

      {/* Rows */}
      <div className="space-y-0.5">
        {edges.map((e: any, i: number) => {
          const hasIssue = e.status === 'degraded' || e.status === 'missing_link' || e.status === 'unreachable';
          return (
            <div key={i}
              onClick={() => openEdge(e)}
              className={cn(
                'flex items-center gap-3 px-4 py-2 rounded-a-sm cursor-pointer transition-colors text-xs',
                'border border-a-border/30 hover:bg-a-border/10',
                hasIssue && 'bg-[#ff5c72]/3 border-l-2 border-l-[#ff5c72]',
              )}>
              <span className="w-28 text-right font-medium text-a-fg truncate">{e.from_node_name}</span>
              <div className="w-20 flex justify-center">
                <div className={cn(
                  'flex items-center gap-1 px-2 py-0.5 rounded text-[10px]',
                  e.status === 'verified' ? 'bg-[#4cd964]/10 text-[#4cd964]' :
                  hasIssue ? 'bg-[#ff5c72]/10 text-[#ff5c72]' :
                  'bg-a-border/20 text-a-muted',
                )}>
                  <span className={cn('w-1.5 h-1.5 rounded-full',
                    e.status === 'verified' ? 'bg-[#4cd964]' : hasIssue ? 'bg-[#ff5c72]' : 'bg-a-muted')} />
                  {e.status === 'verified' ? '通' : e.status === 'degraded' ? '降级' : e.status === 'unreachable' ? '不通' : e.status === 'missing_link' ? '缺Link' : '未知'}
                </div>
              </div>
              <span className="w-28 font-medium text-a-fg truncate">{e.to_node_name}</span>
              <span className={cn('w-14 text-center text-[10px]', e.private_reachable ? 'text-[#4cd964]' : 'text-a-muted')}>
                {e.private_reachable ? '可达' : '—'}
              </span>
              <span className={cn('w-14 text-center text-[10px]', e.public_reachable ? 'text-[#4cd964]' : 'text-a-muted')}>
                {e.public_reachable ? '可达' : '—'}
              </span>
              <span className="flex-1 flex items-center gap-2 min-w-0">
                <StatusBadge status={e.status} />
                {e.gateway_link_id && <span className="text-[10px] font-mono text-a-muted truncate">{e.gateway_link_id}</span>}
              </span>
              <span className="w-40 text-right text-[10px] text-[#ff5c72] truncate" title={e.last_error}>
                {e.last_error || '—'}
              </span>
            </div>
          );
        })}
      </div>

      {/* Detail Drawer */}
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={selected ? `${selected.from_node_name} ↔ ${selected.to_node_name}` : '拓扑边详情'}
        subtitle="节点间连通性"
        width="md"
        footer={
          <Btn onClick={() => { toast('路径测试已触发'); }}>
            测试路径
          </Btn>
        }
      >
        {selected && (
          <div className="space-y-4">
            <Card title="连通性">
              <MetaRow label="私有网" value={selected.private_reachable ? '可达' : '不可达'} />
              <MetaRow label="公网" value={selected.public_reachable ? '可达' : '不可达'} />
              <MetaRow label="验证状态" value={selected.status} />
              <MetaRow label="最后验证" value={selected.last_verified_at ? new Date(selected.last_verified_at).toLocaleString('zh-CN') : '未验证'} />
              {selected.last_error && <MetaRow label="错误" value={selected.last_error} color="text-[#ff5c72]" />}
            </Card>

            <Card title="关联 Gateway Link">
              {linkedLink ? (
                <>
                  <MetaRow label="Link ID" value={linkedLink?.gateway_link_id || '—'} mono />
                  <MetaRow label="状态" value={linkedLink?.status || '—'} />
                  <MetaRow label="创建时间" value={linkedLink && linkedLink.created_at ? new Date(linkedLink.created_at).toLocaleString('zh-CN') : '—'} />
                </>
              ) : (
                <div className="text-xs text-a-muted py-2">
                  {selected.gateway_link_id
                    ? `关联 Link: ${selected.gateway_link_id}（未找到详情）`
                    : '无边关联的 Gateway Link — 这对节点之间没有配置认证通道'}
                </div>
              )}
            </Card>
          </div>
        )}
      </Drawer>
    </div>
  );
}
