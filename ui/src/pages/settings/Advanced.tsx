// ─── Advanced Settings — storage paths, security info ───
import { useQuery } from '@tanstack/react-query';
import { fetchSettings } from '@/lib/api-bridge';
import { Card, PageHeader } from '@/components/shared';

export default function AdvancedSettings() {
  const { data } = useQuery({
    queryKey: ['settings'],
    queryFn: fetchSettings,
  });

  const s = data as any;
  const store = s?.store || {};
  const runtime = s?.runtime || {};

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="高级设置" subtitle="存储路径 · 安全信息" />

      {/* ── Storage ── */}
      <Card title="存储管理" subtitle="程序运行产生的数据与日志路径（只读）">
        <div className="grid grid-cols-1 gap-3 text-xs">
          {/* Core data */}
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-a-accent font-medium">核心业务数据</span>
              <span className="text-[10px] text-a-muted/50">SQLite · 路由/服务/证书/审计</span>
            </div>
            <div className="space-y-1.5">
              <PathRow label="数据库" path={store.sqlite_path} />
              <PathRow label="备份目录" path={store.backup_dir} />
              <PathRow label="数据目录" path={runtime.data_dir} />
              <PathRow label="配置目录" path={runtime.config_dir} />
            </div>
            {store.backup_enabled && (
              <div className="mt-2 text-[10px] text-a-muted">
                自动备份: 每 {store.backup_interval_hrs}h · 保留 {store.backup_keep_count} 份
              </div>
            )}
          </div>

          {/* Logs */}
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-a-fg font-medium">日志系统</span>
              <span className="text-[10px] text-a-muted/50">journald · 操作审计 · 运行日志</span>
            </div>
            <div className="space-y-1.5">
              <PathRow label="系统日志" path="journalctl -u aegis" />
              <PathRow label="操作日志" path="DB: operation_logs 表" />
              <PathRow label="审计日志" path="DB: audit_logs 表" />
              <PathRow label="Apply 日志" path="DB: apply_logs 表" />
            </div>
          </div>
        </div>
      </Card>

      {/* ── Security ── */}
      <Card title="安全信息" subtitle="网关链路和 API 密钥的存储与保护">
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <p className="font-medium text-a-fg mb-1">Gateway Link 密钥</p>
            <p className="text-a-muted">HMAC-SHA256 哈希存储，原始值仅创建时返回一次</p>
          </div>
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <p className="font-medium text-a-fg mb-1">API 密钥</p>
            <p className="text-a-muted">bcrypt 哈希存储，支持 Scope 访问控制</p>
          </div>
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <p className="font-medium text-a-fg mb-1">凭据加密</p>
            <p className="text-a-muted">AES-256-GCM 加密存储，连接字符串不落盘</p>
          </div>
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <p className="font-medium text-a-fg mb-1">Session</p>
            <p className="text-a-muted">HTTP-only Cookie，bcrypt 密码 + rate limiting</p>
          </div>
        </div>
      </Card>
    </div>
  );
}

function PathRow({ label, path }: { label: string; path?: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-a-muted w-16 shrink-0">{label}</span>
      <code className="text-[11px] text-a-fg bg-a-border/10 px-1.5 py-0.5 rounded font-mono truncate select-all">
        {path || '—'}
      </code>
    </div>
  );
}
