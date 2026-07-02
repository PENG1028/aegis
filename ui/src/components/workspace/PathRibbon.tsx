// ─── PathRibbon Component ───
// Core 1: Detail page header showing the full object chain.
// External Request → Entry Point → Listener → Route → Gateway → Service → Endpoint → Node
// Extends the existing PathChain concept with clickable steps and status indicators.

import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { HealthDot } from '@/components/shared/HealthDot';
import { StatusBadge } from '@/components/shared/StatusBadge';
import type { ObjectChain, ChainHealth } from '@/types/workspace';

interface PathRibbonProps {
  chain: ObjectChain;
  focusType?: string;
  focusId?: string;
  className?: string;
}

interface RibbonSegment {
  label: string;
  name: string;
  status: string;
  path?: string;
  isFocused: boolean;
  isPresent: boolean;
}

function chainHealthLabel(status: ChainHealth): string {
  switch (status) {
    case 'healthy': return '链路正常';
    case 'degraded': return '链路降级';
    case 'broken': return '链路中断';
  }
}

function chainHealthColor(status: ChainHealth): string {
  switch (status) {
    case 'healthy': return 'text-[#4cd964]';
    case 'degraded': return 'text-[#e8b830]';
    case 'broken': return 'text-[#ff5c72]';
  }
}

export function PathRibbon({ chain, focusType, focusId, className }: PathRibbonProps) {
  const navigate = useNavigate();

  const segments: RibbonSegment[] = [
    {
      label: '入口',
      name: chain.entryPoint?.domain || '—',
      status: chain.entryPoint?.status || 'unknown',
      path: chain.entryPoint ? `/exposure/entry/${chain.entryPoint.route_id}` : undefined,
      isFocused: focusType === 'entry' || focusType === 'route',
      isPresent: !!chain.entryPoint,
    },
    {
      label: '监听器',
      name: chain.listener ? `:${chain.listener.port}/${chain.listener.provider}` : '—',
      status: chain.listener?.status || 'unknown',
      path: chain.listener ? `/fabric/listeners` : undefined,
      isFocused: focusType === 'listener',
      isPresent: !!chain.listener,
    },
    {
      label: '网关',
      name: chain.gateway?.name || '—',
      status: chain.gateway?.status || 'unknown',
      path: chain.gateway ? `/fabric/gateway/${chain.gateway.gateway_id}` : undefined,
      isFocused: focusType === 'gateway',
      isPresent: !!chain.gateway,
    },
    {
      label: '服务',
      name: chain.service?.name || '—',
      status: chain.service?.health_status || 'unknown',
      path: chain.service ? `/exposure/service/${chain.service.service_id}` : undefined,
      isFocused: focusType === 'service',
      isPresent: !!chain.service,
    },
    {
      label: '端点',
      name: chain.endpoints.length > 0
        ? `${chain.endpoints.length} 个端点`
        : '—',
      status: chain.endpoints.every(e => e.health_status === 'healthy')
        ? 'healthy'
        : chain.endpoints.some(e => e.health_status === 'unhealthy')
          ? 'unhealthy'
          : 'unknown',
      path: chain.service ? `/exposure/service/${chain.service.service_id}` : undefined,
      isFocused: focusType === 'endpoint',
      isPresent: chain.endpoints.length > 0,
    },
    {
      label: '节点',
      name: chain.nodes.length > 0
        ? chain.nodes.map(n => n.name).join(', ')
        : '—',
      status: chain.nodes.every(n => n.status === 'online')
        ? 'healthy'
        : chain.nodes.some(n => n.status === 'offline')
          ? 'unhealthy'
          : 'unknown',
      path: chain.nodes.length === 1 ? `/runtime/node/${chain.nodes[0].node_id}` : undefined,
      isFocused: focusType === 'node',
      isPresent: chain.nodes.length > 0,
    },
  ];

  return (
    <div className={cn('bg-a-surface border border-a-border rounded-a-md overflow-hidden', className)}>
      {/* Chain health indicator */}
      <div className={cn(
        'px-4 py-1.5 flex items-center gap-2 border-b border-a-border text-xs',
        chainHealthColor(chain.status),
      )}>
        <HealthDot status={chain.status === 'healthy' ? 'healthy' : chain.status === 'degraded' ? 'degraded' : 'failed'} />
        <span className="font-medium">{chainHealthLabel(chain.status)}</span>
      </div>

      {/* Segments */}
      <div className="flex items-stretch overflow-x-auto">
        {segments.map((seg, i) => (
          <div key={seg.label} className="flex items-stretch">
            {/* Segment */}
            <button
              onClick={() => seg.path && navigate(seg.path)}
              disabled={!seg.path}
              className={cn(
                'flex flex-col items-center justify-center px-4 py-3 min-w-[90px] transition-colors group',
                seg.isFocused && 'bg-a-accent/10 ring-1 ring-a-accent/30 rounded',
                seg.path && 'cursor-pointer hover:bg-a-border/20',
                !seg.path && 'cursor-default',
                !seg.isPresent && 'opacity-40',
              )}
            >
              <span className="text-[10px] text-a-muted font-medium uppercase tracking-wider mb-1">
                {seg.label}
              </span>
              <span className={cn(
                'text-xs font-mono font-medium truncate max-w-[120px]',
                seg.isFocused ? 'text-a-accent' : 'text-a-fg',
                !seg.isPresent && 'text-a-muted',
              )}>
                {seg.name}
              </span>
              {seg.isPresent && (
                <span className="mt-1">
                  <StatusBadge status={seg.status} />
                </span>
              )}
            </button>
            {/* Arrow connector */}
            {i < segments.length - 1 && (
              <div className="flex items-center text-a-muted/40 px-0.5">
                <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 18l6-6-6-6" />
                </svg>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
