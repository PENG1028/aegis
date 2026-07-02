// ─── RelationshipMap Component ───
// Core 2: Detail page body showing upstream/downstream relationships.
// Upstream: what depends on me (entries, routes, parent services)
// Current: my key properties
// Downstream: what I depend on (gateways, endpoints, nodes, providers)

import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { Card } from '@/components/shared/Card';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { HealthDot } from '@/components/shared/HealthDot';
import type { ObjectChain } from '@/types/workspace';
import type { Endpoint, Node, Gateway, Route, Service } from '@/types';

interface RelationshipMapProps {
  chain: ObjectChain;
  focusType: string;
  focusId: string;
  className?: string;
}

function ObjectCard({
  type,
  id,
  name,
  status,
  detail,
  path,
}: {
  type: string;
  id: string;
  name: string;
  status: string;
  detail?: string;
  path?: string;
}) {
  const navigate = useNavigate();

  return (
    <button
      onClick={() => path && navigate(path)}
      disabled={!path}
      className={cn(
        'flex items-center gap-3 px-3 py-2.5 rounded-a-sm border border-a-border/50',
        'bg-a-bg/50 hover:bg-a-border/20 transition-colors text-left w-full',
        path && 'cursor-pointer',
        !path && 'cursor-default opacity-60',
      )}
    >
      <HealthDot status={status as any} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-a-muted uppercase">{type}</span>
          <span className="text-xs font-medium text-a-fg truncate">{name}</span>
        </div>
        {detail && <p className="text-[10px] text-a-muted truncate mt-0.5">{detail}</p>}
      </div>
      <StatusBadge status={status} />
    </button>
  );
}

export function RelationshipMap({ chain, focusType, focusId, className }: RelationshipMapProps) {
  const isFocused = (type: string, id: string) => type === focusType && id === focusId;

  return (
    <div className={cn('space-y-4', className)}>
      {/* ── Upstream: What depends on me ── */}
      <Card title="上游依赖" subtitle="哪些对象依赖当前对象">
        <div className="space-y-2">
          {chain.entryPoint && !isFocused('entry', chain.entryPoint.route_id) && (
            <ObjectCard
              type="入口"
              id={chain.entryPoint.route_id}
              name={chain.entryPoint.domain}
              status={chain.entryPoint.status}
              path={`/exposure/entry/${chain.entryPoint.route_id}`}
            />
          )}
          {chain.entryPoint && chain.service && !isFocused('service', chain.service.service_id) && (
            <ObjectCard
              type="服务"
              id={chain.service.service_id}
              name={chain.service.name}
              status={chain.service.health_status}
              detail={`${chain.endpoints.length} 个端点 · ${chain.service.latency_ms}ms`}
              path={`/exposure/service/${chain.service.service_id}`}
            />
          )}
          {!chain.entryPoint && !chain.service && (
            <p className="text-xs text-a-muted py-2">无上游依赖</p>
          )}
        </div>
      </Card>

      {/* ── Current: My key properties ── */}
      <Card title="当前对象" subtitle={focusType === 'node' ? '节点详情' : focusType === 'gateway' ? '网关详情' : '对象详情'}>
        <div className="grid grid-cols-2 gap-2">
          {chain.entryPoint && isFocused('entry', chain.entryPoint.route_id) && (
            <>
              <div className="text-xs text-a-muted">域名</div>
              <div className="text-xs font-mono text-a-fg">{chain.entryPoint.domain}</div>
              <div className="text-xs text-a-muted">TLS 模式</div>
              <div className="text-xs text-a-fg">{chain.entryPoint.tls_mode}</div>
              <div className="text-xs text-a-muted">公开访问</div>
              <div className="text-xs text-a-fg">{chain.entryPoint.public_allowed ? '是' : '否'}</div>
            </>
          )}
          {chain.gateway && isFocused('gateway', chain.gateway.gateway_id) && (
            <>
              <div className="text-xs text-a-muted">Provider</div>
              <div className="text-xs text-a-fg">{chain.gateway.provider}</div>
              <div className="text-xs text-a-muted">绑定地址</div>
              <div className="text-xs font-mono text-a-fg">{chain.gateway.bind_addr}:{chain.gateway.port}</div>
              <div className="text-xs text-a-muted">节点</div>
              <div className="text-xs text-a-fg">{chain.gateway.node_name}</div>
            </>
          )}
          {chain.service && isFocused('service', chain.service.service_id) && (
            <>
              <div className="text-xs text-a-muted">类型</div>
              <div className="text-xs text-a-fg">{chain.service.kind}</div>
              <div className="text-xs text-a-muted">上游</div>
              <div className="text-xs font-mono text-a-fg">{chain.service.upstream_url}</div>
              <div className="text-xs text-a-muted">延迟</div>
              <div className="text-xs text-a-fg">{chain.service.latency_ms}ms</div>
            </>
          )}
          {chain.nodes.length > 0 && isFocused('node', chain.nodes[0].node_id) && (
            <>
              <div className="text-xs text-a-muted">IP</div>
              <div className="text-xs font-mono text-a-fg">{chain.nodes[0].public_ip}</div>
              <div className="text-xs text-a-muted">系统</div>
              <div className="text-xs text-a-fg">{chain.nodes[0].os}</div>
              <div className="text-xs text-a-muted">Agent</div>
              <div className="text-xs text-a-fg">{chain.nodes[0].agent_version}</div>
            </>
          )}
        </div>
      </Card>

      {/* ── Downstream: What I depend on ── */}
      <Card title="下游依赖" subtitle="当前对象依赖哪些资源">
        <div className="space-y-2">
          {chain.endpoints.map((ep: Endpoint) => (
            <ObjectCard
              key={ep.endpoint_id}
              type="端点"
              id={ep.endpoint_id}
              name={`${ep.target_local_host}:${ep.target_local_port} (${ep.node_name || ep.node_id})`}
              status={ep.health_status}
              detail={`${ep.protocol} · ${ep.address_type}`}
              path={!isFocused('endpoint', ep.endpoint_id) ? `/exposure/endpoint/${ep.endpoint_id}` : undefined}
            />
          ))}
          {chain.nodes.filter(n => !isFocused('node', n.node_id)).map((node: Node) => (
            <ObjectCard
              key={node.node_id}
              type="节点"
              id={node.node_id}
              name={node.name}
              status={node.status}
              detail={`${node.public_ip} · ${node.agent_version}`}
              path={`/runtime/node/${node.node_id}`}
            />
          ))}
          {chain.gateway && !isFocused('gateway', chain.gateway.gateway_id) && (
            <ObjectCard
              type="网关"
              id={chain.gateway.gateway_id}
              name={chain.gateway.name}
              status={chain.gateway.status}
              detail={`${chain.gateway.provider} · :${chain.gateway.port}`}
              path={`/fabric/gateway/${chain.gateway.gateway_id}`}
            />
          )}
          {chain.endpoints.length === 0 && chain.nodes.length === 0 && !chain.gateway && (
            <p className="text-xs text-a-muted py-2">无下游依赖</p>
          )}
        </div>
      </Card>
    </div>
  );
}
