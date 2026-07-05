// ─── Entry Detail — route/exposure basic info ───
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { routeApi, exposureApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn, useToast } from '@/components/shared';

export default function EntryPointDetail() {
  const { entryId } = useParams<{ entryId: string }>();
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();

  const { data: route, isLoading: rl } = useQuery({
    queryKey: ['route', entryId],
    queryFn: async () => {
      // Try route first, then exposure
      try { const r = await routeApi.get(entryId!); return { ...r, _t: 'route' }; }
      catch { try { const e = await exposureApi.get(entryId!); return { ...e, _t: 'exposure' }; }
      catch { return null; } }
    },
    enabled: !!entryId,
  });

  const disableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/disable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route', entryId] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const enableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/enable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route', entryId] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });

  if (rl) return <div className="p-6 text-a-muted text-sm">加载中...</div>;
  if (!route) return <div className="p-6 text-a-muted text-sm">未找到入口 {entryId}</div>;

  const item = route as any;
  const isRoute = item._t === 'route';
  const active = item.status === 'active';

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">{isRoute ? item.domain : `:${item.entry_port || item.port || '?'}`}</h2>
          <p className="text-xs text-a-muted mt-1">{isRoute ? (item.tls_enabled ? 'HTTPS Route' : 'HTTP Route') : (item.type || 'TCP/UDP').toUpperCase()} · <StatusBadge status={active ? 'active' : 'disabled'} /></p>
        </div>
        <div className="flex gap-2">
          {isRoute && (
            active
              ? <Btn onClick={() => disableMutation.mutate()} className="text-[10px]">禁用</Btn>
              : <Btn onClick={() => enableMutation.mutate()} className="text-[10px]">启用</Btn>
          )}
          <Btn onClick={() => nav('/exposure')} className="text-[10px]">返回列表</Btn>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card title="基本信息">
          <div className="space-y-2 text-xs">
            <div className="flex justify-between"><span className="text-a-muted">类型</span><span className="font-mono">{isRoute ? (item.tls_enabled ? 'HTTPS' : 'HTTP') : (item.type || 'TCP').toUpperCase()}</span></div>
            <div className="flex justify-between"><span className="text-a-muted">状态</span><StatusBadge status={active ? 'active' : 'disabled'} /></div>
            <div className="flex justify-between"><span className="text-a-muted">来源</span><span className="font-mono">{item.owner_type === 'space' ? (item.space_id || item.owner_id || 'service') : '管理员'}</span></div>
            {isRoute && <div className="flex justify-between"><span className="text-a-muted">Service ID</span><span className="font-mono text-[10px]">{item.service_id || '—'}</span></div>}
            {item.gateway_link_id && <div className="flex justify-between"><span className="text-a-muted">Gateway Link</span><span className="font-mono text-[10px]">{item.gateway_link_id}</span></div>}
            <div className="flex justify-between"><span className="text-a-muted">路径</span><span className="font-mono">{item.path_prefix || '/'}</span></div>
          </div>
        </Card>

        {!isRoute && (
          <Card title="端口信息">
            <div className="space-y-2 text-xs">
              <div className="flex justify-between"><span className="text-a-muted">入口端口</span><span className="font-mono">{item.entry_port || item.port || '?'}</span></div>
              <div className="flex justify-between"><span className="text-a-muted">协议</span><span className="font-mono">{item.type || item.exposure_type || 'TCP'}</span></div>
              <div className="flex justify-between"><span className="text-a-muted">目标</span><span className="font-mono">{item.target_host || '?'}:{item.target_port || '?'}</span></div>
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}
