// ─── Advanced Settings ───
// Consolidated: SecurityPage, MaintenancePage, ActionsPage

import { Card, PageHeader, Btn } from '@/components/shared';

export default function AdvancedSettings() {
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="高级设置" subtitle="安全 · 维护 · 操作" />

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

      <Card title="维护操作" subtitle="备份、WAL 检查点、快照">
        <div className="grid grid-cols-3 gap-2">
          {['备份数据库', 'WAL 检查点', '配置快照', '导出诊断', '一致性检查', '清理日志'].map(item => (
            <button
              key={item}
              disabled
              className="p-3 rounded-a-sm border border-a-border bg-a-bg text-xs text-a-muted text-center cursor-not-allowed opacity-50"
            >
              {item}
            </button>
          ))}
        </div>
        <p className="text-[10px] text-a-muted mt-3">维护操作将在后续版本中接入 API</p>
      </Card>

      <Card title="受控操作" subtitle="绑定域名 · TLS · 更新目标">
        <div className="grid grid-cols-3 gap-2">
          {['绑定 HTTP 域名', '绑定 TLS 后端', '更新目标', '禁用域名', '删除域名', '中继测试'].map(item => (
            <button
              key={item}
              className="p-3 rounded-a-sm border border-a-border bg-a-bg text-xs text-a-fg text-center hover:bg-a-border/20 transition-colors cursor-pointer"
            >
              {item}
            </button>
          ))}
        </div>
      </Card>
    </div>
  );
}
