import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchJoinTokens, createJoinToken, revokeJoinToken } from '@/lib/api-bridge';
import {
  PageHeader, Card, DataTable, StatusBadge,
  Alert, WarningCard,
} from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { JoinToken } from '@/types';
import { fmtDate } from '@/lib/utils';

export default function JoinTokensPage() {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [revealedToken, setRevealedToken] = useState<string | null>(null);
  const [form, setForm] = useState({ name: '', roles: 'gateway', expiresDays: 7 });

  const { data, isLoading, error } = useQuery({
    queryKey: ['join-tokens'],
    queryFn: fetchJoinTokens,
  });

  const createMutation = useMutation({
    mutationFn: () => createJoinToken({
      name: form.name,
      allowed_roles: form.roles.split(',').map((r) => r.trim()),
      allowed_source_cidr: null,
      expires_at: new Date(Date.now() + form.expiresDays * 86400000).toISOString(),
      expected_node_name: null,
    }),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['join-tokens'] });
      setRevealedToken(result.rawToken || '');
      setShowCreate(false);
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeJoinToken(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['join-tokens'] }),
  });

  const columns: DataTableColumn<JoinToken>[] = [
    { key: 'name', label: '名称' },
    { key: 'token_prefix', label: 'Token Prefix', mono: true, muted: true },
    {
      key: 'allowed_roles',
      label: 'Roles',
      render: (row) => row.allowed_roles.join(', '),
    },
    {
      key: 'expires_at',
      label: '过期',
      mono: true,
      render: (row) => fmtDate(row.expires_at),
    },
    {
      key: 'status',
      label: '状态',
      render: (row) => <StatusBadge status={row.status} />,
    },
    {
      key: 'actions',
      label: '操作',
      render: (row) =>
        row.status === 'active' ? (
          <button
            className="text-[11px] text-a-danger hover:underline bg-transparent border-none cursor-pointer"
            onClick={() => {
              if (confirm('确定撤销此 Join Token？')) revokeMutation.mutate(row.id);
            }}
          >
            撤销
          </button>
        ) : null,
    },
  ];

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader
        title="Join Tokens"
        subtitle="节点注册令牌管理"
        helpKey="join-tokens"
        actions={
          <>
            
            <button
              className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-accent text-white hover:opacity-90 cursor-pointer border-none font-medium"
              onClick={() => setShowCreate(true)}
            >
              + 创建
            </button>
          </>
        }
      />

      <Alert type="info">
        Join Token 用于新节点注册到控制面。raw token 仅在创建时显示一次。刷新后不可恢复。
      </Alert>

      <Card>
        <DataTable columns={columns} data={data || []} emptyMessage="暂无 Join Token" keyExtractor={(r) => r.id} />
      </Card>

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center" onClick={(e) => { if (e.target === e.currentTarget) setShowCreate(false); }}>
          <div className="bg-a-surface border border-a-border rounded-a-lg shadow-2xl w-[90%] max-w-md">
            <div className="flex items-center justify-between px-5 py-4 border-b border-a-border">
              <h3 className="text-sm font-semibold">创建 Join Token</h3>
              <button className="text-a-muted hover:text-a-fg text-lg bg-transparent border-none cursor-pointer" onClick={() => setShowCreate(false)}>×</button>
            </div>
            <div className="p-5 space-y-3">
              <div>
                <label className="block text-xs font-medium text-a-muted mb-1">名称</label>
                <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="e.g. node-c join" />
              </div>
              <div>
                <label className="block text-xs font-medium text-a-muted mb-1">Allowed Roles</label>
                <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent" value={form.roles} onChange={(e) => setForm({ ...form, roles: e.target.value })} placeholder="gateway, relay_target" />
              </div>
              <div>
                <label className="block text-xs font-medium text-a-muted mb-1">过期天数</label>
                <input type="number" className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent" value={form.expiresDays} onChange={(e) => setForm({ ...form, expiresDays: parseInt(e.target.value) || 7 })} />
              </div>
            </div>
            <div className="flex gap-2 justify-end px-5 py-3 border-t border-a-border">
              <button className="text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer" onClick={() => setShowCreate(false)}>取消</button>
              <button
                className="text-xs px-3 py-1.5 rounded-a-md bg-a-accent text-white hover:opacity-90 cursor-pointer border-none font-medium"
                onClick={() => createMutation.mutate()}
                disabled={!form.name || createMutation.isPending}
              >
                {createMutation.isPending ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Raw Token Reveal */}
      {revealedToken && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center">
          <div className="bg-a-surface border border-a-border rounded-a-lg shadow-2xl w-[90%] max-w-md">
            <div className="flex items-center justify-between px-5 py-4 border-b border-a-border">
              <h3 className="text-sm font-semibold text-a-warn">⚠ Raw Token — 仅显示一次</h3>
              <button className="text-a-muted hover:text-a-fg text-lg bg-transparent border-none cursor-pointer" onClick={() => setRevealedToken(null)}>×</button>
            </div>
            <div className="p-5">
              <WarningCard title="立即复制并保存" type="warn">
                <p>此 token 不会再次显示。刷新页面后无法恢复。</p>
              </WarningCard>
              <div className="mt-3 font-mono text-sm bg-a-bg border border-a-border rounded-a-sm p-3 break-all select-all text-a-accent">
                {revealedToken}
              </div>
              <button
                className="mt-3 w-full text-xs py-2 rounded-a-md bg-a-accent text-white hover:opacity-90 cursor-pointer border-none font-medium"
                onClick={() => {
                  navigator.clipboard.writeText(revealedToken);
                  setRevealedToken(null);
                }}
              >
                已复制，关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
