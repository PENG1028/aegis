import { Card, PageHeader, Btn } from '@/components/shared';
import Input from '@/components/ui/Input';
import { useState } from 'react';

export default function QuickConnect() {
  const [domain, setDomain] = useState('');
  const [port, setPort] = useState('80');
  const [node, setNode] = useState('');

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="快速接入" subtitle="一键创建域名 → 服务 → 端点 → 路由" />
      <Card title="新建域名映射">
        <div className="space-y-3 max-w-md">
          <div><label className="text-xs text-a-muted block mb-1">域名</label><Input value={domain} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDomain(e.target.value)} placeholder="api.example.com" /></div>
          <div><label className="text-xs text-a-muted block mb-1">端口</label><Input value={port} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPort(e.target.value)} placeholder="80" /></div>
          <div><label className="text-xs text-a-muted block mb-1">目标节点</label><Input value={node} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNode(e.target.value)} placeholder="node-a" /></div>
          <Btn primary>创建并预览</Btn>
        </div>
      </Card>
    </div>
  );
}
