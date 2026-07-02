// ─── ImpactPanel Component ───
// Core 3: Shows affected objects before write operations.
// Rendered inside Drawer (medium risk) or Wizard (high risk).

import { cn } from '@/lib/utils';
import { HealthDot } from '@/components/shared/HealthDot';
import type { ImpactScope, AffectedObject } from '@/types/impact';

interface ImpactPanelProps {
  impact: ImpactScope | null;
  loading?: boolean;
  className?: string;
}

function AffectedGroup({
  title,
  items,
}: {
  title: string;
  items: AffectedObject[];
}) {
  if (items.length === 0) return null;

  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-a-fg2">{title}</span>
        <span className="text-[10px] px-1.5 py-0.5 rounded bg-a-border/50 text-a-muted">
          {items.length}
        </span>
      </div>
      {items.map((item, i) => (
        <div
          key={`${item.type}-${item.id}-${i}`}
          className={cn(
            'flex items-center gap-2 px-3 py-1.5 rounded-a-sm text-xs',
            item.impact === 'direct'
              ? 'bg-[#ff5c72]/5 border border-[#ff5c72]/10'
              : 'bg-a-border/10 border border-a-border/20',
          )}
        >
          <HealthDot
            status={
              item.status === 'healthy' ? 'healthy'
              : item.status === 'degraded' ? 'degraded'
              : item.status === 'failed' || item.status === 'unhealthy' ? 'failed'
              : 'unknown'
            }
          />
          <span className="text-[10px] uppercase text-a-muted w-12 shrink-0">{item.type}</span>
          <span className="font-medium text-a-fg truncate flex-1">{item.name}</span>
          <span className="text-[10px] text-a-muted">
            {item.impact === 'direct' ? '直接影响' : '间接影响'}
          </span>
        </div>
      ))}
    </div>
  );
}

export function ImpactPanel({ impact, loading, className }: ImpactPanelProps) {
  if (loading) {
    return (
      <div className={cn('text-center py-6 text-a-muted text-sm', className)}>
        正在计算影响范围...
      </div>
    );
  }

  if (!impact) {
    return (
      <div className={cn('text-center py-6 text-a-muted text-sm', className)}>
        暂无法计算影响范围
      </div>
    );
  }

  const hasAny = impact.totalAffected > 0;

  return (
    <div className={cn('space-y-4', className)}>
      {/* Summary */}
      <div className="flex items-center gap-3 p-3 rounded-a-md bg-a-bg border border-a-border">
        <div className="text-2xl">
          {impact.hasDownstreamFailures ? '⚠️' : hasAny ? 'ℹ️' : '✅'}
        </div>
        <div>
          <p className="text-sm font-medium text-a-fg">
            {impact.hasDownstreamFailures
              ? '检测到下游故障'
              : hasAny
                ? `此操作将影响 ${impact.totalAffected} 个对象`
                : '此操作没有检测到影响范围'}
          </p>
          <p className="text-xs text-a-muted mt-0.5">
            操作: {impact.operation} · 目标: {impact.target.name}
          </p>
        </div>
      </div>

      {/* Affected groups */}
      {hasAny && (
        <div className="space-y-3">
          <AffectedGroup title="入口 / 路由" items={impact.affectedEntries} />
          <AffectedGroup title="服务" items={impact.affectedServices} />
          <AffectedGroup title="网关" items={impact.affectedGateways} />
          <AffectedGroup title="节点" items={impact.affectedNodes} />
        </div>
      )}

      {/* Downstream failure warning */}
      {impact.hasDownstreamFailures && (
        <div className="p-3 rounded-a-md bg-[#ff5c72]/10 border border-[#ff5c72]/20">
          <p className="text-xs text-[#ff5c72] font-medium">
            ⚠️ 部分受影响对象当前处于故障状态，继续操作可能导致服务中断。
          </p>
        </div>
      )}
    </div>
  );
}
