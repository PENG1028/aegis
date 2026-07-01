import { PageHeader, Card, Btn, Alert } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

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
