// ─── TLS Settings ───
import { Card, PageHeader } from '@/components/shared';

export default function TlsSettings() {
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="TLS 证书" subtitle="上传和管理自定义 TLS 证书" />
      <Card title="证书管理">
        <div className="text-center py-8 text-a-muted text-sm">
          <div className="text-3xl mb-3 opacity-30">🔒</div>
          <p>TLS 证书管理</p>
          <p className="text-xs mt-1 opacity-60">支持上传自定义证书或使用 Let's Encrypt 自动管理</p>
        </div>
      </Card>
    </div>
  );
}
