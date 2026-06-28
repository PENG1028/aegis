/**
 * QuickCreate — 一步创建域名映射。
 *
 * 填域名 + 端口 + 节点 → 自动创建 Service + Endpoint + Route → Apply。
 * 高级选项（TLS、GatewayPolicy 等）折叠在底部。
 */

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchNodes, adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export default function QuickCreatePage() {
  const toast = useToast();

  const [domain, setDomain] = useState('');
  const [port, setPort] = useState('3000');
  const [targetHost, setTargetHost] = useState('127.0.0.1');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [creating, setCreating] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  // Fetch nodes for dropdown
  const { data: nodes } = useQuery({
    queryKey: ['nodes-for-quickcreate'],
    queryFn: () => fetchNodes(),
  });

  async function doCreate() {
    if (!domain.trim() || !port.trim() || !targetHost.trim()) {
      toast('域名、目标主机和端口不能为空', 'error');
      return;
    }
    setCreating(true);
    setError(null);
    setResult(null);
    try {
      // Use the Action API to create everything in one step
      await adminApi.bindHTTPDomain({
        domain: domain.trim(),
        target_host: targetHost.trim(),
        target_port: parseInt(port),
      });
      toast('创建完成！请执行 Apply 生效');
      setResult({ domain: domain.trim(), target: `${targetHost.trim()}:${port}`, status: 'created' });
    } catch (e: any) {
      setError(e.message);
      toast(e.message, 'error');
    }
    setCreating(false);
  }

  return (
    <div>
      <PageHeader title="创建映射" helpKey="quick-create"
        sub="一步创建域名到端口的映射 — 自动处理 Service + Endpoint + Route" />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card title="基本配置">
          <div className="p-[18px] space-y-4">
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1.5">域名</label>
              <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                value={domain} onChange={(e) => setDomain(e.target.value)}
                placeholder="例：blog.example.com" autoFocus />
            </div>
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="block text-xs font-medium text-a-muted mb-1.5">目标主机</label>
                <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={targetHost} onChange={(e) => setTargetHost(e.target.value)}
                  placeholder="127.0.0.1" />
              </div>
              <div className="w-24">
                <label className="block text-xs font-medium text-a-muted mb-1.5">端口</label>
                <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={port} onChange={(e) => setPort(e.target.value)}
                  placeholder="3000" />
              </div>
            </div>
            <div className="pt-2">
              <Btn primary onClick={doCreate} disabled={creating} className="w-full justify-center">
                {creating ? '创建中…' : '创建映射'}
              </Btn>
            </div>
          </div>
        </Card>

        <Card title="说明">
          <div className="p-[18px] space-y-3 text-xs text-a-fg2">
            <p>创建映射会执行以下操作：</p>
            <ol className="list-decimal list-inside space-y-2">
              <li>创建 <strong>Service</strong>（后端服务定义）</li>
              <li>创建 <strong>Endpoint</strong>（监听地址和端口）</li>
              <li>创建 <strong>Route</strong>（域名 → Service 的映射）</li>
            </ol>
            <p className="text-a-warn mt-3">⚠ 映射创建后需要执行 <strong>Apply</strong> 才能推送到网关生效。</p>
          </div>
        </Card>
      </div>

      {error && <Alert type="err" className="mt-4">{error}</Alert>}

      {/* 高级选项（折叠） */}
      <button className="flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mt-3 mb-2 bg-transparent border-none cursor-pointer"
        onClick={() => setShowAdvanced(!showAdvanced)}>
        <span className={`inline-block transition-transform ${showAdvanced ? 'rotate-90' : ''}`}>▶</span>
        高级选项（TLS、网关策略等）
      </button>
      {showAdvanced && (
        <Card className="mb-4">
          <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1">目标节点</label>
              <select className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none">
                {(nodes || []).map((n: any) => (
                  <option key={n.node_id} value={n.node_id}>{n.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1">TLS 模式</label>
              <select className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none">
                <option value="http_only">仅 HTTP</option>
                <option value="terminate_local">本地终止</option>
                <option value="passthrough_deferred">直通（延期）</option>
              </select>
            </div>
            <div>
              <label className="flex items-center gap-2">
                <input type="checkbox" defaultChecked className="accent-a-accent" />
                <span>允许公网访问</span>
              </label>
            </div>
            <div>
              <label className="flex items-center gap-2">
                <input type="checkbox" defaultChecked className="accent-a-accent" />
                <span>Preserve Host</span>
              </label>
            </div>
          </div>
        </Card>
      )}

      {/* 创建结果 */}
      {result && (
        <Card title="创建完成" className="mt-4">
          <div className="p-[18px]">
            <div className="text-xs text-a-success mb-3">✓ 映射已创建到数据库</div>
            <table className="text-xs w-full">
              <tbody>
                <tr><td className="text-a-muted pr-4 py-1">域名</td><td className="font-mono">{result.domain}</td></tr>
                <tr><td className="text-a-muted pr-4 py-1">目标</td><td className="font-mono">{result.target}</td></tr>
              </tbody>
            </table>
            <div className="mt-3 flex gap-2">
              <Btn sm onClick={() => window.location.href = '/apply'}>去 Apply</Btn>
              <Btn sm onClick={() => window.location.href = '/routes'}>查看路由列表</Btn>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}
