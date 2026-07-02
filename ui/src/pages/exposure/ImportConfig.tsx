import { Card, PageHeader, Btn } from '@/components/shared';
export default function ImportConfig() {
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="导入配置" subtitle="从 Caddyfile 导入路由" />
      <Card title="Caddyfile 导入">
        <div className="text-center py-8 text-a-muted text-sm">
          <div className="text-3xl mb-3 opacity-30">📥</div>
          <p>扫描服务器上的 Caddyfile，预览提取的路由，选择性导入</p>
          <Btn primary className="mt-4">扫描 Caddyfile</Btn>
        </div>
      </Card>
    </div>
  );
}
