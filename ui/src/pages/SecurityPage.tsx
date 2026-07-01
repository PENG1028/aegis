import { PageHeader, Card, Alert, MetaRow } from '@/components/shared';

export function SecurityPage() {
  return (
    <div>
      <PageHeader title="安全 / 密钥边界" helpKey="security" sub="Token 存储和密钥边界" />
      <Alert type="warn">此页面不显示 raw token。仅展示存储状态和密钥边界。</Alert>

      <div className="grid grid-cols-2 gap-4">
        <Card title="GatewayLink 令牌存储">
          <div className="p-[18px]">
            <MetaRow label="存储方式" value="encrypted (AES-256-GCM)" />
            <MetaRow label="HMAC 签名" value="SHA-256" />
            <MetaRow label="轮换" value="supported" />
            <MetaRow label="密钥边界" value="内存中解密，不落盘" />
          </div>
        </Card>
        <Card title="API 密钥存储">
          <div className="p-[18px]">
            <MetaRow label="Storage" value="SHA-256 hashed" />
            <MetaRow label="管理会话" value="cookie-based" />
            <MetaRow label="令牌轮换" value="supported" />
            <MetaRow label="审计日志" value="enabled" />
          </div>
        </Card>
      </div>
    </div>
  );
}
