// ─── Entry Detail — route info + health checks ───
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { runtimeModeApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

export default function EntryPointDetail() {
  const { entryId } = useParams<{ entryId: string }>();
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();

  // Fetch route via admin API
  const { data: route, isLoading: rl } = useQuery({
    queryKey: ['route-detail', entryId],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}`, { credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!entryId,
  });

  // Fetch runtime mode for health checks
  const { data: rm } = useQuery({
    queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get(), refetchInterval: 60_000,
  });

  const disableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/disable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const enableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/enable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });

  if (rl) return <div className="p-6 text-a-muted text-sm">加载中...</div>;
  if (!route) return <div className="p-6 text-a-muted text-sm">未找到入口 {entryId}</div>;

  const item = route as any;
  const active = item.status === 'active';
  const tls = item.tls_enabled;
  const compName = tls ? 'HTTPS Route' : 'HTTP Route';

  // Health checks
  const compositions = rm?.current?.compositions || [];
  const comp = compositions.find((c: any) => c.name === compName);
  const entryHealthy = comp?.status === 'available';
  const mode = rm?.current?.label || 'Legacy';

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">{item.domain}</h2>
          <p className="text-xs text-a-muted mt-1">{compName} · <StatusBadge status={active ? 'active' : 'disabled'} /> · {mode} 模式</p>
        </div>
        <div className="flex gap-2">
          {active
            ? <Btn onClick={() => disableMutation.mutate()} className="text-[10px]">禁用</Btn>
            : <Btn onClick={() => enableMutation.mutate()} className="text-[10px]">启用</Btn>}
          <Btn onClick={() => nav('/exposure')} className="text-[10px]">返回列表</Btn>
        </div>
      </div>

      {/* Health */}
      <Card title="健康检查">
        <div className="space-y-2">
          {/* Entry health */}
          <div className={cn('flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs',
            entryHealthy ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15')}>
            <span className={cn('font-mono text-sm shrink-0', entryHealthy ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
              {entryHealthy ? '✓' : '✗'}
            </span>
            <span className="font-medium w-20 shrink-0">入口健康</span>
            <span className={entryHealthy ? 'text-a-muted' : 'text-[#ff5c72]/80'}>
              {entryHealthy
                ? `${compName} 可用（${mode} 模式，Provider 已就绪）`
                : `${compName} 需要 Caddy 提供路由能力（${mode} 模式，Caddy 未安装）`}
            </span>
          </div>

          {/* Egress health — show endpoint info */}
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs bg-a-border/5 border-a-border/20">
            <span className="font-mono text-sm shrink-0 text-a-muted">—</span>
            <span className="font-medium w-20 shrink-0">出口健康</span>
            <span className="text-a-muted">
              {item.service_id
                ? `目标服务: ${item.service_id}（端点状态需在服务详情页查看）`
                : '无关联服务'}
            </span>
          </div>
        </div>
      </Card>

      {/* Basic info */}
      <div className="grid grid-cols-2 gap-4">
        <Card title="基本信息">
          <div className="space-y-2 text-xs">
            <Row label="域名" value={item.domain} />
            <Row label="类型" value={compName} />
            <Row label="状态" badge={<StatusBadge status={active ? 'active' : 'disabled'} />} />
            <Row label="来源" value={item.owner_type === 'space' ? (item.space_id || 'service') : '管理员'} />
            <Row label="Service ID" value={item.service_id || '—'} mono />
            {item.gateway_link_id && <Row label="Gateway Link" value={item.gateway_link_id} mono />}
            <Row label="路径" value={item.path_prefix || '/'} mono />
          </div>
        </Card>

        <Card title="模式信息">
          <div className="space-y-2 text-xs">
            <Row label="当前模式" value={mode} />
            <Row label="组合能力" value={compName} />
            <Row label="Provider" value={entryHealthy ? 'Caddy 已就绪' : 'Caddy 未安装'} />
            <Row label="TLS" value={tls ? '开启（:443）' : '关闭（:80）'} />
            <Row label="创建时间" value={item.created_at || '—'} mono />
          </div>
        </Card>
      </div>
    </div>
  );
}

function Row({ label, value, badge, mono }: { label: string; value?: string; badge?: any; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-a-muted">{label}</span>
      {badge || <span className={mono ? 'font-mono text-[11px]' : ''}>{value || '—'}</span>}
    </div>
  );
}
