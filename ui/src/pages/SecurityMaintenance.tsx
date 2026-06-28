import { useState } from 'react';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, MetaRow, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export function SecurityPage() {
  const toast = useToast();

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

export function MaintenancePage() {
  const toast = useToast();

  const items = [
    { name: 'SQLite 备份', status: 'available', desc: '备份 SQLite 数据库' },
    { name: 'WAL 检查点', status: 'available', desc: '执行 WAL checkpoint 压缩日志' },
    { name: '诊断导出', status: 'available', desc: '导出 Provider/Listener/Node 诊断数据' },
    { name: '配置快照', status: 'available', desc: '创建当前 config snapshot' },
    { name: '回滚快照', status: 'disabled', desc: '回滚到上一个 config snapshot' },
    { name: '从备份恢复', status: 'disabled', desc: '从 SQLite 备份恢复' },
  ];

  return (
    <div>
      <PageHeader title="系统维护" helpKey="maintenance" sub="系统维护操作" />
      <Alert type="info">当前为界面原型。实际操作需要真实环境支持。</Alert>

      <div className="grid grid-cols-2 gap-4">
        {items.map((item) => (
          <Card key={item.name} title={item.name}
            actions={<Btn sm disabled={item.status === 'disabled'}>{item.status === 'disabled' ? '不可用' : '执行'}</Btn>}>
            <div className="p-[18px]">
              <MetaRow label="状态" value={item.status} />
              <div className="text-xs text-a-muted mt-1">{item.desc}</div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

export function ActionsPage() {
  const toast = useToast();

  const acts = [
    { key: 'bind_http', title: '绑定 HTTP 域名', desc: '绑定域名到 HTTP target' },
    { key: 'bind_tls', title: '绑定 TLS 后端', desc: '绑定 SNI host' },
    { key: 'update_target', title: '更新目标', desc: '更新 target 地址' },
    { key: 'relay_test', title: '中继解析测试', desc: '测试 relay 路径解析' },
  ];

  return (
    <div>
      <PageHeader title="操作" helpKey="actions" sub="通过受控 Action 修改网关资源" />
      <Alert type="info">Actions 提供目标性操作，避免直接编辑配置。</Alert>
      <div className="grid grid-cols-2 gap-4">
        {acts.map((a) => (
          <Card key={a.key} title={a.title}
            actions={<Btn primary sm onClick={() => toast(`${a.title}: 已提交 (mock)`)}>执行</Btn>}>
            <div className="p-[18px] text-xs text-a-muted">{a.desc}</div>
          </Card>
        ))}
      </div>
    </div>
  );
}
