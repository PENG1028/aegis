// ─── Entry List — routes + exposures + managed domains ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { routeApi, exposureApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn } from '@/components/shared';

function EmptyState({ title, desc, onNew }: { title: string; desc: string; onNew: () => void }) {
  return (
    <div className="text-center py-10 text-a-muted text-sm border border-dashed border-a-border/30 rounded-a-md">
      <p className="font-medium text-a-fg mb-1">{title}</p>
      <p className="text-xs opacity-60 mb-3">{desc}</p>
      <Btn onClick={onNew}>新建入口</Btn>
    </div>
  );
}

export default function EntryList() {
  const nav = useNavigate();

  const { data: rd, isLoading: rl } = useQuery({
    queryKey: ['routes'], queryFn: () => routeApi.list().catch(() => ({ routes: [] })), refetchInterval: 30_000,
  });
  const { data: ed, isLoading: el } = useQuery({
    queryKey: ['exposures'], queryFn: () => exposureApi.list().catch(() => ({ exposures: [] })), refetchInterval: 30_000,
  });

  const routes = (rd as any)?.routes || [];
  const exposures = (ed as any)?.exposures || [];

  const httpItems = routes.map((r: any) => ({
    key: r.id, domain: r.domain, comp: r.composition || (r.tls_enabled ? 'HTTPS' : 'HTTP'),
    target: r.target || '—', status: r.status,
  }));
  const tcpItems = exposures.map((e: any) => ({
    key: e.id, domain: `:${e.entry_port || e.port || '?'}`,
    comp: (e.type || 'TCP').toUpperCase(), target: `${e.target_host || '?'}:${e.target_port || '?'}`, status: e.status,
  }));

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">流量管理</h2>
          <p className="text-xs text-a-muted mt-1">{routes.length + exposures.length} 个入口</p>
        </div>
        <Btn primary onClick={() => nav('/exposure/new')}>新建入口</Btn>
      </div>

      {/* 域名入口 */}
      <Card title={`域名入口 (${httpItems.length})`} subtitle="HTTP/HTTPS 域名映射，按 Host 头路由">
        {rl ? <div className="text-sm text-a-muted py-6 text-center">加载中...</div>
         : httpItems.length === 0
           ? <EmptyState title="暂无域名入口" desc="创建域名映射，让外部通过域名访问你的服务" onNew={() => nav('/exposure/new')} />
           : <Table items={httpItems} onRowClick={key => nav(`/exposure/entry/${key}`)} />}
      </Card>

      {/* 端口入口 */}
      <Card title={`端口入口 (${tcpItems.length})`} subtitle="TCP/UDP 端口转发，按端口号路由">
        {el ? <div className="text-sm text-a-muted py-6 text-center">加载中...</div>
         : tcpItems.length === 0
           ? <EmptyState title="暂无端口入口" desc="创建端口转发，用于 Redis/MySQL/自定义 TCP 服务" onNew={() => nav('/exposure/new')} />
           : <Table items={tcpItems} onRowClick={key => nav(`/exposure/entry/${key}`)} />}
      </Card>
    </div>
  );
}

function Table({ items, onRowClick }: { items: any[]; onRowClick: (key: string) => void }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead><tr className="border-b border-a-border text-a-muted text-left">
          <th className="py-2.5 px-3 font-medium">入口</th>
          <th className="py-2.5 px-3 font-medium">类型</th>
          <th className="py-2.5 px-3 font-medium">后端</th>
          <th className="py-2.5 px-3 font-medium">状态</th>
        </tr></thead>
        <tbody>{items.map(item => (
          <tr key={item.key} className="border-b border-a-border/30 hover:bg-a-border/5 cursor-pointer" onClick={() => onRowClick(item.key)}>
            <td className="py-2.5 px-3 font-mono text-[11px]">{item.domain}</td>
            <td className="py-2.5 px-3 text-[10px] text-a-muted">{item.comp}</td>
            <td className="py-2.5 px-3 font-mono text-[11px] text-a-muted">{item.target}</td>
            <td className="py-2.5 px-3"><StatusBadge status={item.status === 'active' ? 'active' : 'disabled'} /></td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  );
}
