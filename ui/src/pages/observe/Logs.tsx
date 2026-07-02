// ─── Logs / Audit ───
import { useState } from 'react';
import { useLocation } from 'react-router-dom';
import { Card, PageHeader, TabBar, Timestamp, StatusBadge } from '@/components/shared';

const MOCK_OPS = [
  { action: 'apply_config', detail: '推送配置 v43 → 3 条路由', created_at: '2026-07-02T10:30:00Z', status: 'success' },
  { action: 'create_route', detail: '创建路由 docs.proofnote.dev', created_at: '2026-07-02T09:15:00Z', status: 'success' },
  { action: 'disable_endpoint', detail: '禁用端点 endpoint-auth-b', created_at: '2026-07-02T08:00:00Z', status: 'warning' },
  { action: 'rotate_key', detail: '轮换 API 密钥 key-1', created_at: '2026-07-01T16:00:00Z', status: 'success' },
  { action: 'create_gateway_link', detail: '创建网关链路 link-main-private', created_at: '2026-07-01T14:00:00Z', status: 'success' },
];

const MOCK_AUDIT = [
  { action: 'login', detail: '管理员 admin 登录', created_at: '2026-07-02T10:29:00Z', status: 'success' },
  { action: 'reveal_credential', detail: '揭示凭据 pg-db', created_at: '2026-07-02T09:00:00Z', status: 'warning' },
  { action: 'delete_route', detail: '删除路由 old.example.com', created_at: '2026-07-01T12:00:00Z', status: 'success' },
];

const TABS = [
  { key: 'ops', label: '操作日志' },
  { key: 'audit', label: '审计日志' },
];

export default function Logs() {
  const location = useLocation();
  const initialTab = location.pathname.includes('audit') ? 'audit' : 'ops';
  const [tab, setTab] = useState(initialTab);

  const entries = tab === 'audit' ? MOCK_AUDIT : MOCK_OPS;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title={tab === 'audit' ? '审计日志' : '操作日志'} subtitle={`${entries.length} 条记录`} />
      <TabBar tabs={TABS} active={tab} onChange={setTab} />
      <Card>
        <div className="space-y-1">
          {entries.map((e, i) => (
            <div key={i} className="flex items-center gap-3 px-3 py-2.5 rounded-a-sm hover:bg-a-border/10 text-xs">
              <Timestamp iso={e.created_at} />
              <span className="font-mono text-a-fg2 w-36 truncate">{e.action}</span>
              <span className="text-a-fg flex-1 truncate">{e.detail}</span>
              <StatusBadge status={e.status} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
