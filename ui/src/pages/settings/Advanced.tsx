// ─── Advanced Settings ───
// Security info, quick links to common operations.
// Maintenance operations removed — they were disabled stubs.

import { Card, PageHeader } from '@/components/shared';

export default function AdvancedSettings() {
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="高级设置" subtitle="安全 · 管理操作" />

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
