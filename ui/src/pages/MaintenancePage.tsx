import { PageHeader, Card, Btn, Alert, MetaRow } from '@/components/shared';

export function MaintenancePage() {
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
