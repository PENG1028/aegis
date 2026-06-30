/**
 * Credentials — 凭据管理 (v1.8K)
 *
 * 管理加密的连接串:
 *   postgres://user:pass@host:port/db
 *   mysql://user:pass@host:port/db
 *   redis://:pass@host:port/0
 *   ws://host:port/path
 *
 * 连接串使用 AES-256-GCM 加密存储（与 GatewayLink 相同的模式）。
 * 创建后凭据别名可在 TCP Exposure 中通过 credential://alias 引用。
 */

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { credentialApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

const SCHEME_LABELS: Record<string, string> = {
  postgres: 'PostgreSQL', postgresql: 'PostgreSQL', mysql: 'MySQL',
  mariadb: 'MariaDB', redis: 'Redis', rediss: 'Redis TLS',
  mongodb: 'MongoDB', ws: 'WebSocket', wss: 'WSS',
};
const SCHEME_COLORS: Record<string, string> = {
  postgres: 'text-[#336791]', postgresql: 'text-[#336791]',
  mysql: 'text-[#00758f]', mariadb: 'text-[#00758f]',
  redis: 'text-[#dc382d]', rediss: 'text-[#dc382d]',
  mongodb: 'text-[#4db33d]', ws: 'text-[#4a9eff]', wss: 'text-[#4a9eff]',
};

const SCHEME_PLACEHOLDERS: Record<string, string> = {
  postgres: 'postgres://user:password@host:5432/dbname',
  mysql: 'mysql://user:password@host:3306/dbname',
  redis: 'redis://:password@host:6379/0',
  mongodb: 'mongodb://user:password@host:27017/dbname',
  ws: 'ws://host:8080/path',
};

export default function CredentialsPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [alias, setAlias] = useState('');
  const [connString, setConnString] = useState('');
  const [description, setDescription] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['credentials'],
    queryFn: () => credentialApi.list(),
    refetchInterval: 60_000,
  });

  const createMu = useMutation({
    mutationFn: () => credentialApi.create(alias, connString, description),
    onSuccess: () => {
      toast('凭据已创建');
      setShowCreate(false); setAlias(''); setConnString(''); setDescription('');
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
    },
    onError: (e: any) => toast(e.message || '创建失败', 'error'),
  });

  const deleteMu = useMutation({
    mutationFn: (id: string) => credentialApi.delete(id),
    onSuccess: () => {
      toast('凭据已删除');
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
    },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const credentials: any[] = data?.credentials || [];

  return (
    <div>
      <PageHeader
        title="凭据"
        helpKey="credentials"
        sub={`${credentials.length} 个加密连接串`}
        actions={
          <Btn primary onClick={() => setShowCreate(!showCreate)}>
            {showCreate ? '取消' : '添加凭据'}
          </Btn>
        }
      />

      {/* ─── Create form ─── */}
      {showCreate && (
        <Card title="添加凭据" className="mb-4">
          <div className="p-4 space-y-3">
            <div>
              <label className="block text-[10px] text-a-muted mb-1">别名 (供 TCP Exposure 引用)</label>
              <input
                className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                value={alias}
                onChange={(e) => setAlias(e.target.value)}
                placeholder="例如: pg-prod, redis-cache"
              />
              <div className="text-[10px] text-a-muted mt-0.5">
                创建后在 Exposure 中通过 <code className="text-a-accent">credential://{alias || '别名'}</code> 引用
              </div>
            </div>
            <div>
              <label className="block text-[10px] text-a-muted mb-1">连接串 (加密存储，创建后不可查看)</label>
              <textarea
                className="w-full font-mono text-[11px] px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent resize-y"
                rows={3}
                value={connString}
                onChange={(e) => setConnString(e.target.value)}
                placeholder={SCHEME_PLACEHOLDERS[connString.split(':')[0]] || 'postgres://user:password@host:5432/db'}
                spellCheck={false}
              />
            </div>
            <div>
              <label className="block text-[10px] text-a-muted mb-1">备注 (可选)</label>
              <input
                className="w-full text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="生产数据库 / 缓存 / ..."
              />
            </div>
            <div className="flex gap-2">
              <Btn primary onClick={() => createMu.mutate()} disabled={createMu.isPending || !alias || !connString}>
                {createMu.isPending ? '创建中…' : '创建并加密'}
              </Btn>
              <Btn onClick={() => { setShowCreate(false); setAlias(''); setConnString(''); setDescription(''); }}>取消</Btn>
            </div>
          </div>
        </Card>
      )}

      {/* ─── Loading / Empty ─── */}
      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">加载中…</div>}
      {!isLoading && credentials.length === 0 && (
        <div className="text-center py-16 text-a-muted text-xs space-y-2">
          <div>暂无凭据</div>
          <div className="text-[10px]">点击「添加凭据」创建加密连接串，然后在 TCP Exposure 中引用</div>
        </div>
      )}

      {/* ─── Credential list ─── */}
      <div className="space-y-2">
        {credentials.map((c: any) => (
          <Card key={c.id}>
            <div className="p-4 flex items-center gap-4">
              {/* Scheme badge */}
              <span className={`text-xs font-mono font-bold ${SCHEME_COLORS[c.scheme] || 'text-a-muted'}`}>
                {SCHEME_LABELS[c.scheme] || c.scheme || 'TCP'}
              </span>

              {/* Info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-mono font-medium">{c.alias}</span>
                  <span className="text-[10px] text-a-muted">v{c.secret_version}</span>
                </div>
                <div className="text-[10px] text-a-muted font-mono mt-0.5 truncate">
                  {c.masked_uri}
                </div>
                {c.description && (
                  <div className="text-[10px] text-a-muted mt-0.5">{c.description}</div>
                )}
              </div>

              {/* Ref usage hint */}
              <code className="text-[10px] text-a-accent bg-a-accent/5 px-1.5 py-0.5 rounded-a-sm">
                credential://{c.alias}
              </code>

              {/* Actions */}
              <Btn
                onClick={() => {
                  if (confirm(`删除凭据 "${c.alias}"？依赖它的 Exposure 将无法解析目标。`)) {
                    deleteMu.mutate(c.id);
                  }
                }}
                disabled={deleteMu.isPending}
              >
                删除
              </Btn>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
