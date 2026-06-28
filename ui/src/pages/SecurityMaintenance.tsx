import { useState } from 'react';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, MetaRow, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export function SecurityPage() {
  const toast = useToast();

  return (
    <div>
      <PageHeader title="Security / Secret Boundary" helpKey="security" sub="Token 存储和密钥边界" helpKey="doctor" />
      <Alert type="warn">此页面不显示 raw token。仅展示存储状态和密钥边界。</Alert>

      <div className="grid grid-cols-2 gap-4">
        <Card title="GatewayLink Token Storage">
          <div className="p-[18px]">
            <MetaRow label="Storage" value="encrypted (AES-256-GCM)" />
            <MetaRow label="HMAC Signing" value="SHA-256" />
            <MetaRow label="Rotation" value="supported" />
            <MetaRow label="Secret Boundary" value="内存中解密，不落盘" />
          </div>
        </Card>
        <Card title="API Key Storage">
          <div className="p-[18px]">
            <MetaRow label="Storage" value="SHA-256 hashed" />
            <MetaRow label="Admin Session" value="cookie-based" />
            <MetaRow label="Token Rotation" value="supported" />
            <MetaRow label="Audit Logging" value="enabled" />
          </div>
        </Card>
      </div>
    </div>
  );
}

export function MaintenancePage() {
  const toast = useToast();

  const items = [
    { name: 'SQLite Backup', status: 'available', desc: '备份 SQLite 数据库' },
    { name: 'WAL Checkpoint', status: 'available', desc: '执行 WAL checkpoint 压缩日志' },
    { name: 'Diagnostics Export', status: 'available', desc: '导出 Provider/Listener/Node 诊断数据' },
    { name: 'Config Snapshot', status: 'available', desc: '创建当前 config snapshot' },
    { name: 'Rollback Snapshot', status: 'disabled', desc: '回滚到上一个 config snapshot' },
    { name: 'Restore from Backup', status: 'disabled', desc: '从 SQLite 备份恢复' },
  ];

  return (
    <div>
      <PageHeader title="Maintenance" sub="系统维护操作" helpKey="doctor" />
      <Alert type="info">当前为界面原型。实际操作需要真实环境支持。</Alert>

      <div className="grid grid-cols-2 gap-4">
        {items.map((item) => (
          <Card key={item.name} title={item.name}
            actions={<Btn sm disabled={item.status === 'disabled'}>{item.status === 'disabled' ? '不可用' : '执行'}</Btn>}>
            <div className="p-[18px]">
              <MetaRow label="Status" value={item.status} />
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
    { key: 'bind_http', title: 'Bind HTTP Domain', desc: '绑定域名到 HTTP target' },
    { key: 'bind_tls', title: 'Bind TLS Backend', desc: '绑定 SNI host' },
    { key: 'update_target', title: 'Update Target', desc: '更新 target 地址' },
    { key: 'relay_test', title: 'Relay Resolve Test', desc: '测试 relay 路径解析' },
  ];

  return (
    <div>
      <PageHeader title="Actions" helpKey="actions" sub="通过受控 Action 修改网关资源" helpKey="doctor" />
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
