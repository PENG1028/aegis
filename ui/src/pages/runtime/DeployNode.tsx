import { Card, PageHeader, Btn } from '@/components/shared';
import Input from '@/components/ui/Input';
import { useState } from 'react';

export default function DeployNode() {
  const [ssh, setSsh] = useState('');
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="部署节点" subtitle="一键远程部署新节点" />
      <Card title="SSH 部署">
        <div className="space-y-3 max-w-md">
          <div><label className="text-xs text-a-muted block mb-1">SSH 地址</label><Input value={ssh} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSsh(e.target.value)} placeholder="ubuntu@43.160.211.232" /></div>
          <div><label className="text-xs text-a-muted block mb-1">Join Token</label><Input placeholder="选择或创建加入令牌" /></div>
          <Btn primary>部署</Btn>
        </div>
      </Card>
    </div>
  );
}
