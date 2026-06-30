/**
 * QuickCreate — 一步创建域名映射。
 *
 * 填域名 + 端口 + 节点 → 自动创建 Service + Endpoint + Route → Apply。
 * 高级选项（TLS、GatewayPolicy 等）折叠在底部。
 *
 * v1.8J: 整合 Apply 一键流程 — 创建完成后可直接推送部署。
 */

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchNodes, adminApi, system } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export default function QuickCreatePage() {
  const toast = useToast();
  const queryClient = useQueryClient();

  const [domain, setDomain] = useState('');
  const [port, setPort] = useState('3000');
  const [targetHost, setTargetHost] = useState('127.0.0.1');
  const [targetNode, setTargetNode] = useState('');
  const [tlsMode, setTlsMode] = useState('http_only');
  const [publicAccess, setPublicAccess] = useState(true);
  const [preserveHost, setPreserveHost] = useState(true);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [creating, setCreating] = useState(false);
  const [applying, setApplying] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [applyResult, setApplyResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  // Fetch nodes for dropdown
  const { data: nodes } = useQuery({
    queryKey: ['nodes-for-quickcreate'],
    queryFn: () => fetchNodes(),
  });

  // Fetch system status for pending state
  const { data: sysStatus } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => system.status(),
  });

  async function doCreate() {
    if (!domain.trim() || !port.trim() || !targetHost.trim()) {
      toast('域名、目标主机和端口不能为空', 'error');
      return;
    }
    setCreating(true);
    setError(null);
    setResult(null);
    setApplyResult(null);
    try {
      // Use the Action API to create everything in one step
      await adminApi.bindHTTPDomain({
        domain: domain.trim(),
        target_host: targetHost.trim(),
        target_port: parseInt(port),
        target_node: targetNode || undefined,
        tls_mode: tlsMode,
        public_access: publicAccess,
        preserve_host: preserveHost,
      });
      toast('创建完成！');
      setResult({ domain: domain.trim(), target: `${targetHost.trim()}:${port}`, status: 'created' });
    } catch (e: any) {
      setError(e.message);
      toast(e.message, 'error');
    }
    setCreating(false);
  }

  async function doCreateAndApply() {
    if (!domain.trim() || !port.trim() || !targetHost.trim()) {
      toast('域名、目标主机和端口不能为空', 'error');
      return;
    }
    setCreating(true);
    setError(null);
    setResult(null);
    setApplyResult(null);
    try {
      // Step 1: Create
      await adminApi.bindHTTPDomain({
        domain: domain.trim(),
        target_host: targetHost.trim(),
        target_port: parseInt(port),
        target_node: targetNode || undefined,
        tls_mode: tlsMode,
        public_access: publicAccess,
        preserve_host: preserveHost,
      });
      setResult({ domain: domain.trim(), target: `${targetHost.trim()}:${port}`, status: 'created' });
      toast('创建完成，正在推送配置…');

      // Step 2: Apply
      setCreating(false);
      setApplying(true);
      try {
        const applyRes = await adminApi.applyConfig();
        setApplyResult(applyRes);
        toast(applyRes.message || '推送完成！');
        queryClient.invalidateQueries({ queryKey: ['system-status'] });
      } catch (e: any) {
        setApplyResult({ error: e.message });
        toast('推送失败: ' + e.message, 'error');
      }
    } catch (e: any) {
      setError(e.message);
      toast(e.message, 'error');
    }
    setCreating(false);
    setApplying(false);
  }

  const hasPending = sysStatus?.pending_apply?.pending;

  return (
    <div>
      <PageHeader title="创建映射" helpKey="quick-create"
        sub="一步创建域名到端口的映射 — 自动处理 Service + Endpoint + Route" />

      {/* Pending apply banner */}
      {hasPending && (
        <Alert type="warn" className="mb-4">
          <span className="font-medium">有待推送的变更。</span>
          <span className="ml-2">创建新映射后可以一并推送。</span>
        </Alert>
      )}

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
            <div className="flex gap-2 pt-2">
              <Btn primary onClick={doCreate} disabled={creating || applying} className="flex-1 justify-center">
                {creating && !applying ? '创建中…' : '创建映射'}
              </Btn>
              <Btn primary onClick={doCreateAndApply} disabled={creating || applying} className="flex-1 justify-center">
                {applying ? '推送中…' : creating ? '创建中…' : '创建并推送'}
              </Btn>
            </div>
          </div>
        </Card>

        <Card title="流程说明">
          <div className="p-[18px] space-y-3 text-xs text-a-fg2">
            <p><strong>创建映射</strong> — 仅创建资源到数据库，需手动执行 Apply。</p>
            <p><strong>创建并推送</strong> — 创建后自动执行 Apply，一步到位。</p>
            <div className="mt-3 pt-3 border-t border-a-border space-y-2">
              <p className="text-a-muted">完整流程：</p>
              <ol className="list-decimal list-inside space-y-1 text-a-muted">
                <li>创建 <strong>Service</strong>（后端服务定义）</li>
                <li>创建 <strong>Endpoint</strong>（监听地址和端口）</li>
                <li>创建 <strong>Route</strong>（域名 → Service 的映射）</li>
                <li>执行 <strong>Apply</strong> → 生成 Caddyfile → 验证 → 重载</li>
              </ol>
            </div>
            <p className="text-a-warn mt-2">⚠ 推送后需验证端点健康状态，建议前往 <a href="/health" className="text-a-accent">健康检查</a> 确认。</p>
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
              <select className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
                value={targetNode} onChange={(e) => setTargetNode(e.target.value)}>
                <option value="">自动选择</option>
                {(nodes || []).map((n: any) => (
                  <option key={n.node_id} value={n.node_id}>{n.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1">TLS 模式</label>
              <select className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
                value={tlsMode} onChange={(e) => setTlsMode(e.target.value)}>
                <option value="http_only">仅 HTTP</option>
                <option value="terminate_local">本地终止</option>
                <option value="passthrough_deferred">直通（延期）</option>
              </select>
            </div>
            <div>
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={publicAccess} onChange={(e) => setPublicAccess(e.target.checked)} className="accent-a-accent" />
                <span>允许公网访问</span>
              </label>
            </div>
            <div>
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={preserveHost} onChange={(e) => setPreserveHost(e.target.checked)} className="accent-a-accent" />
                <span>Preserve Host</span>
              </label>
            </div>
          </div>
        </Card>
      )}

      {/* 创建结果 */}
      {result && (
        <Card title={applyResult && !applyResult.error ? '创建 & 推送完成' : '创建完成'} className="mt-4">
          <div className="p-[18px]">
            <div className={`text-xs mb-3 ${applyResult && !applyResult.error ? 'text-a-success' : 'text-a-accent'}`}>
              {applyResult && !applyResult.error ? '✓ 映射已创建并推送至网关' : '✓ 映射已创建到数据库'}
            </div>
            <table className="text-xs w-full mb-3">
              <tbody>
                <tr><td className="text-a-muted pr-4 py-1">域名</td><td className="font-mono">{result.domain}</td></tr>
                <tr><td className="text-a-muted pr-4 py-1">目标</td><td className="font-mono">{result.target}</td></tr>
                {applyResult && !applyResult.error && (
                  <>
                    <tr><td className="text-a-muted pr-4 py-1">推送状态</td><td className="font-mono text-a-success">{applyResult.message || applyResult.status || 'success'}</td></tr>
                    {applyResult.routes != null && <tr><td className="text-a-muted pr-4 py-1">路由数</td><td className="font-mono">{applyResult.routes}</td></tr>}
                    {applyResult.warnings != null && <tr><td className="text-a-muted pr-4 py-1">警告</td><td className="font-mono text-a-warn">{applyResult.warnings}</td></tr>}
                  </>
                )}
                {applyResult?.error && (
                  <tr><td className="text-a-muted pr-4 py-1">推送错误</td><td className="font-mono text-a-danger">{applyResult.error}</td></tr>
                )}
              </tbody>
            </table>
            <div className="flex gap-2 flex-wrap">
              {!applyResult && (
                <Btn sm onClick={() => window.location.href = '/apply'}>去 Apply 推送</Btn>
              )}
              <Btn sm onClick={() => window.location.href = '/routes'}>查看路由列表</Btn>
              <Btn sm onClick={() => window.location.href = '/health'}>健康检查</Btn>
              <Btn sm onClick={() => {
                setResult(null);
                setApplyResult(null);
                setDomain('');
                setPort('3000');
                setTargetHost('127.0.0.1');
              }}>继续创建</Btn>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}
