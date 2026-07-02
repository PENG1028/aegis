// ─── ReleaseDiffViewer Component ───
// Core 4: Configuration release diff viewer.
// Human-readable summary + raw config diff with line-level coloring.

import { useState } from 'react';
import { cn } from '@/lib/utils';
import { TabBar } from '@/components/shared/TabBar';
import type { ReleaseDiff, DiffChange } from '@/types/diff';

interface ReleaseDiffViewerProps {
  diff: ReleaseDiff | null;
  loading?: boolean;
  className?: string;
}

function SummaryView({ diff }: { diff: ReleaseDiff }) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4 text-xs">
        <div className="flex items-center gap-2">
          <span className="text-a-muted">从</span>
          <code className="px-1.5 py-0.5 rounded bg-a-bg text-a-fg font-mono">{diff.versionFrom}</code>
        </div>
        <svg className="w-4 h-4 text-a-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M5 12h14M12 5l7 7-7 7" />
        </svg>
        <div className="flex items-center gap-2">
          <span className="text-a-muted">到</span>
          <code className="px-1.5 py-0.5 rounded bg-a-bg text-a-fg font-mono">{diff.versionTo}</code>
        </div>
      </div>

      <p className="text-sm text-a-fg2">{diff.summary}</p>

      <div className="space-y-2">
        {diff.changes.map((change, i) => (
          <ChangeCard key={i} change={change} />
        ))}
      </div>
    </div>
  );
}

function ChangeCard({ change }: { change: DiffChange }) {
  const typeConfig = {
    added: { label: '新增', color: 'text-[#4cd964]', bg: 'bg-[#4cd964]/5', border: 'border-[#4cd964]/20' },
    removed: { label: '删除', color: 'text-[#ff5c72]', bg: 'bg-[#ff5c72]/5', border: 'border-[#ff5c72]/20' },
    modified: { label: '修改', color: 'text-[#e8b830]', bg: 'bg-[#e8b830]/5', border: 'border-[#e8b830]/20' },
  };

  const config = typeConfig[change.type];

  return (
    <div className={cn('p-3 rounded-a-sm border', config.bg, config.border)}>
      <div className="flex items-center gap-2 mb-1.5">
        <span className={cn('text-[10px] font-bold uppercase px-1.5 py-0.5 rounded', config.bg, config.color)}>
          {config.label}
        </span>
        {change.domain && (
          <span className="text-xs font-mono text-a-fg">{change.domain}</span>
        )}
      </div>
      <p className="text-xs text-a-fg2">{change.description}</p>
      {change.before && (
        <div className="mt-2 p-2 rounded bg-[#ff5c72]/5 border border-[#ff5c72]/10 text-xs font-mono text-[#ff5c72]">
          - {change.before}
        </div>
      )}
      {change.after && (
        <div className="mt-1 p-2 rounded bg-[#4cd964]/5 border border-[#4cd964]/10 text-xs font-mono text-[#4cd964]">
          + {change.after}
        </div>
      )}
    </div>
  );
}

function RawDiffView({ diff }: { diff: ReleaseDiff }) {
  if (!diff.rawDiff) {
    return <p className="text-xs text-a-muted py-4 text-center">无原始配置差异</p>;
  }

  const lines = diff.rawDiff.split('\n');

  return (
    <div className="bg-a-bg border border-a-border rounded-a-sm overflow-hidden">
      <div className="px-3 py-1.5 border-b border-a-border flex items-center gap-2">
        <span className="text-[10px] text-a-muted uppercase">Caddyfile Diff</span>
        <span className="text-[10px] text-a-muted">
          {lines.length} 行
        </span>
      </div>
      <div className="overflow-x-auto">
        <pre className="text-xs font-mono leading-relaxed p-3">
          {lines.map((line, i) => {
            let lineClass = 'text-a-fg2';
            if (line.startsWith('+')) lineClass = 'text-[#4cd964]';
            else if (line.startsWith('-')) lineClass = 'text-[#ff5c72]';
            else if (line.startsWith('@@')) lineClass = 'text-a-accent';

            return (
              <div key={i} className={cn('flex', line.startsWith('+') && 'bg-[#4cd964]/5', line.startsWith('-') && 'bg-[#ff5c72]/5')}>
                <span className="w-8 text-right text-a-muted select-none pr-3 shrink-0">{i + 1}</span>
                <span className={lineClass}>{line}</span>
              </div>
            );
          })}
        </pre>
      </div>
    </div>
  );
}

const TABS = [
  { key: 'summary', label: '变更摘要' },
  { key: 'raw', label: '原始差异' },
];

export function ReleaseDiffViewer({ diff, loading, className }: ReleaseDiffViewerProps) {
  const [tab, setTab] = useState('summary');

  if (loading) {
    return (
      <div className={cn('text-center py-10 text-a-muted text-sm', className)}>
        正在加载 Diff...
      </div>
    );
  }

  if (!diff) {
    return (
      <div className={cn('text-center py-10 text-a-muted text-sm', className)}>
        暂无配置差异
      </div>
    );
  }

  return (
    <div className={cn('space-y-4', className)}>
      <TabBar
        tabs={TABS}
        active={tab}
        onChange={setTab}
      />
      {tab === 'summary' ? <SummaryView diff={diff} /> : <RawDiffView diff={diff} />}
    </div>
  );
}
