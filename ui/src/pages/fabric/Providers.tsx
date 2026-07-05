// ─── Runtime Binding Matrix ───
// Mode-based: Legacy / EdgeMux / (future modes greyed)
// Unified syntax: listening_at → action → target
// Cell status: active | [MISSING] | [CONFLICT] | [OPTIONAL] | —
//
// v1.8L-20: All mode definitions now come from GET /api/system/runtime-mode.
// No hand-coded data — the backend is the single source of truth.

import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { providerApi, runtimeModeApi } from '@/lib/api-bridge';
import type { RuntimeModeDef, RuntimeAtom, AtomSlot, ProviderAtoms, Composition } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge, Card, Btn, Drawer, TabBar, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

// ══════════════════════════════════════════════════════════════════════════════
// Types
// ══════════════════════════════════════════════════════════════════════════════

interface ProviderState {
  id: string; name: string; gateway_type: string; status: string;
  installed: boolean; running: boolean; version: string;
  binary_path: string; config_path: string;
  capabilities: string[]; theoretical_capabilities: string[];
  ports: { port: number; owner: string; protocol: string; purpose: string; status: string }[];
  diagnostic?: any;
}

interface CapabilityDef {
  key: string; layer: string; label: string; description: string;
}

interface PortPolicy {
  mode: string; bindings: { port: number; owner: string; protocol: string; purpose: string; status: string }[];
}

// ══════════════════════════════════════════════════════════════════════════════
// Cell evaluation (atom-first: one cell = []AtomSlot, not a single binding)
// ══════════════════════════════════════════════════════════════════════════════

type CellStatus = 'active' | 'missing' | 'conflict' | 'optional' | 'inactive' | 'na';

interface EvaluatedCell {
  slots: AtomSlot[];
  status: CellStatus;
  providerId: string;
  providerName: string;
  atom: RuntimeAtom;
  problem?: string;
  impact?: string;
}

function evaluateAtomSlots(
  slots: AtomSlot[] | undefined,
  provider: ProviderState | undefined,
  providerId: string,
  atom: RuntimeAtom,
): EvaluatedCell {
  const base = {
    slots: slots || [],
    providerId,
    providerName: provider?.name || providerId,
    atom,
  };

  if (!slots || slots.length === 0) {
    return { ...base, status: 'na', problem: undefined, impact: undefined };
  }

  const installed = provider?.installed && provider?.running;
  const hasRequired = slots.some(s => s.required);

  if (!installed) {
    if (!hasRequired) {
      return {
        ...base,
        status: 'optional',
        problem: `${provider?.name || providerId} 未安装（可选原子）`,
        impact: `可选能力 ${atom.label} 不可用，不影响核心功能`,
      };
    }
    const name = provider?.name || providerId;
    return {
      ...base,
      status: 'missing',
      problem: `${name} 未安装`,
      impact: `${atom.label} 无法提供，依赖此原子的流量受阻`,
    };
  }

  return { ...base, status: 'active', problem: undefined, impact: undefined };
}

const STATUS_STYLES: Record<CellStatus, { cell: string; badge: string; dot: string }> = {
  active:   { cell: 'bg-[#4cd964]/5 border-[#4cd964]/15', badge: 'bg-[#4cd964]/10 text-[#4cd964]', dot: 'bg-[#4cd964]' },
  missing:  { cell: 'bg-[#ff5c72]/8 border-[#ff5c72]/20', badge: 'bg-[#ff5c72]/15 text-[#ff5c72]', dot: 'bg-[#ff5c72]' },
  conflict: { cell: 'bg-[#e8b830]/8 border-[#e8b830]/20', badge: 'bg-[#e8b830]/15 text-[#e8b830]', dot: 'bg-[#e8b830]' },
  optional: { cell: 'bg-a-border/5 border-a-border/15',   badge: 'bg-a-border/15 text-a-muted',    dot: 'bg-a-border' },
  inactive: { cell: 'bg-a-bg/30 border-a-border/10',       badge: 'bg-a-border/10 text-a-muted/50', dot: 'bg-a-border/50' },
  na:       { cell: 'bg-transparent',                       badge: '',                                dot: '' },
};

function statusLabel(s: CellStatus): string {
  switch (s) {
    case 'active': return '活跃';
    case 'missing': return 'MISSING';
    case 'conflict': return 'CONFLICT';
    case 'optional': return 'OPTIONAL';
    case 'inactive': return '—';
    case 'na': return '—';
  }
}

// ══════════════════════════════════════════════════════════════════════════════
// Sub-components
// ══════════════════════════════════════════════════════════════════════════════

function SlotRow({ s }: { s: AtomSlot }) {
  if (s.port > 0) {
    // L4/L5 atom — has port binding
    return (
      <div className="text-[10px] text-a-fg font-mono leading-relaxed flex items-center gap-1 flex-wrap">
        <span className="text-a-muted">{s.listening_at || `:${s.port}/${s.protocol}`}</span>
        <span className="text-a-border">→</span>
        <span className="text-a-accent font-medium">{s.action}</span>
        <span className="text-a-border">→</span>
        <span className="text-a-fg2">{s.target}</span>
        {s.note && <span className="text-[9px] text-a-muted">({s.note})</span>}
        {!s.required && <span className="text-[9px] text-a-border/60">opt</span>}
      </div>
    );
  }
  // L7 atom — protocol route, no port binding
  return (
    <div className="text-[10px] text-a-fg font-mono leading-relaxed flex items-center gap-1 flex-wrap">
      <span className="text-a-accent font-medium">{s.action}</span>
      <span className="text-a-border">→</span>
      <span className="text-a-fg2">{s.target}</span>
      {s.note && <span className="text-[9px] text-a-muted">({s.note})</span>}
      {!s.required && <span className="text-[9px] text-a-border/60">opt</span>}
    </div>
  );
}

function CellContent({ ev }: { ev: EvaluatedCell }) {
  const st = STATUS_STYLES[ev.status];
  const isMissing = ev.status === 'missing' || ev.status === 'conflict';

  if (ev.status === 'na') {
    return <span className="text-a-muted/30 text-center text-lg">—</span>;
  }

  return (
    <div className="space-y-1">
      {/* Status badge */}
      <div className="flex items-center gap-1.5">
        {st.dot && <span className={cn('w-1.5 h-1.5 rounded-full shrink-0', st.dot)} />}
        <span className={cn('text-[10px] font-medium', isMissing ? 'text-[#ff5c72]' : 'text-a-muted')}>
          {statusLabel(ev.status)}
        </span>
      </div>
      {/* Atom slots — one row per port binding */}
      {ev.slots.map((s, i) => (
        <SlotRow key={i} s={s} />
      ))}
      {/* Problem hint */}
      {isMissing && ev.problem && (
        <div className="text-[10px] text-[#ff5c72]/80">{ev.problem}</div>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Composition bar — clickable cards above the atom matrix
// ══════════════════════════════════════════════════════════════════════════════

const COMP_CARD_STYLE: Record<string, { card: string; text: string; cursor: string }> = {
  available:          { card: 'bg-a-surface border-a-border/30 hover:border-[#4cd964]/40 hover:bg-[#4cd964]/5',     text: 'text-a-fg',            cursor: 'cursor-pointer' },
  missing_provider:   { card: 'bg-[#ff5c72]/5 border-[#ff5c72]/30 hover:border-[#ff5c72]/50 hover:bg-[#ff5c72]/10', text: 'text-[#ff5c72]',       cursor: 'cursor-pointer' },
  unsupported:        { card: 'bg-transparent border-a-border/10',                                                     text: 'text-a-muted/40',      cursor: 'cursor-not-allowed' },
};

function CompositionBar({ compositions, activeAtoms, onHover }: {
  compositions: Composition[];
  activeAtoms: Set<string>;
  onHover: (atoms: string[] | null) => void;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap mb-1">
      <span className="text-[10px] text-a-muted uppercase tracking-wider mr-1">组合流</span>
      {compositions.map(comp => {
        const st = COMP_CARD_STYLE[comp.status] || COMP_CARD_STYLE.unsupported;
        const disabled = comp.status === 'unsupported';
        const title = comp.status === 'unsupported'
          ? `${comp.name}：此模式不支持`
          : comp.status === 'missing_provider'
            ? `${comp.name}：Provider 未安装`
            : `${comp.name} = ${(comp.atoms || []).join(' → ')}`;

        return (
          <button
            key={comp.name}
            disabled={disabled}
            onMouseEnter={() => !disabled && onHover(comp.atoms)}
            onMouseLeave={() => onHover(null)}
            title={title}
            className={cn('px-2.5 py-1 rounded-a-sm text-[10px] font-mono transition-colors border', st.card, st.cursor)}>
            <span className={st.text}>{comp.name}</span>
            <span className={cn('ml-1.5', comp.status === 'unsupported' ? 'text-a-muted/30' : comp.status === 'missing_provider' ? 'text-[#ff5c72]/60' : 'text-a-muted')}>
              {comp.chain || '—'}
            </span>
          </button>
        );
      })}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Mode switcher (top tabs)
// ══════════════════════════════════════════════════════════════════════════════

function ModeSwitcher({ active, onChange, modes }: { active: string; onChange: (id: string) => void; modes: RuntimeModeDef[] }) {
  if (!modes.length) return null;
  return (
    <div className="flex items-center gap-1 flex-wrap">
      {modes.map(m => (
        <button key={m.id} onClick={() => m.implemented && onChange(m.id)}
          disabled={!m.implemented}
          title={m.implemented ? m.description : `${m.label}（即将推出）`}
          className={cn(
            'px-3 py-1.5 rounded-a-sm text-xs transition-colors cursor-pointer',
            m.implemented
              ? (active === m.id
                  ? 'bg-a-accent/20 text-a-accent font-semibold border border-a-accent/30'
                  : 'bg-a-bg text-a-muted hover:text-a-fg hover:bg-a-border/20 border border-a-border/30')
              : 'bg-a-bg/30 text-a-muted/30 cursor-not-allowed border border-a-border/10',
          )}>
          {m.label}
          {!m.implemented && <span className="ml-1 text-[9px] opacity-50">soon</span>}
        </button>
      ))}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Atom Binding Matrix — atoms as columns, grouped by layer (L4 地基 → L5 → L7)
// ══════════════════════════════════════════════════════════════════════════════

const LAYER_HEADER: Record<string, { label: string; color: string }> = {
  L4: { label: 'L4 地基', color: 'text-blue-400' },
  L5: { label: 'L5 安全', color: 'text-teal-400' },
  L7: { label: 'L7 应用', color: 'text-green-400' },
};

// ══════════════════════════════════════════════════════════════════════════════
// Composition Diagnostic — 组合能力可用性诊断表
// ══════════════════════════════════════════════════════════════════════════════

type DiagStatus = 'available' | 'missing-provider' | 'unsupported' | 'degraded' | 'optional-only';

interface AtomDiag {
  atomKey: string;
  atomLabel: string;
  status: 'ok' | 'missing-provider' | 'no-provider' | 'optional';
  providers: string[]; // which providers claim this atom
  missingProviders?: string[]; // providers that claim but aren't installed
}

interface CompDiag {
  composition: Composition;
  overall: DiagStatus;
  atoms: AtomDiag[];
  missingProviderNames: string[];
  fix: string;
}

const DIAG_STATUS_STYLE: Record<DiagStatus, { badge: string; text: string; label: string }> = {
  'available':       { badge: 'bg-[#4cd964]/10 text-[#4cd964]', text: 'text-[#4cd964]', label: '可用' },
  'missing-provider':{ badge: 'bg-[#ff5c72]/10 text-[#ff5c72]', text: 'text-[#ff5c72]', label: '缺 Provider' },
  'unsupported':     { badge: 'bg-a-border/10 text-a-muted',      text: 'text-a-muted',    label: '不支持' },
  'degraded':        { badge: 'bg-[#e8b830]/10 text-[#e8b830]',   text: 'text-[#e8b830]', label: '降级可用' },
  'optional-only':   { badge: 'bg-a-border/10 text-a-muted',      text: 'text-a-muted',    label: '仅可选' },
};

function CompositionDiagnostic({ compositions, providerRows, providers, atoms }: {
  compositions: Composition[];
  providerRows: ProviderAtoms[];
  providers: ProviderState[];
  atoms: RuntimeAtom[];
}) {
  const atomLabelMap = useMemo(() => {
    const m = new Map<string, string>();
    atoms.forEach(a => m.set(a.key, a.label));
    return m;
  }, [atoms]);

  const diags = useMemo((): CompDiag[] => {
    return compositions.map(comp => {
      // Empty atoms = mode doesn't support this composition
      if (!comp.atoms || comp.atoms.length === 0) {
        return {
          composition: comp,
          overall: 'unsupported' as DiagStatus,
          atoms: [],
          missingProviderNames: [],
          fix: '切换到支持此能力的模式（如 EdgeMux）',
        };
      }

      const atomDiags: AtomDiag[] = comp.atoms.map(atomKey => {
        const atomLabel = atomLabelMap.get(atomKey) || atomKey;

        const providingRows = providerRows.filter(row => {
          const slots = row.bindings?.[atomKey];
          return slots && slots.length > 0;
        });

        // No provider claims this atom at all
        if (providingRows.length === 0) {
          return {
            atomKey, atomLabel,
            status: 'no-provider' as const,
            providers: [],
          };
        }

        const providerIds = providingRows.map(r => r.provider_id);

        // Check if all slots are optional
        const allSlots = providingRows.flatMap(r => r.bindings?.[atomKey] || []);
        const hasRequired = allSlots.some(s => s.required);
        if (!hasRequired) {
          return {
            atomKey, atomLabel,
            status: 'optional' as const,
            providers: providerIds,
          };
        }

        // Check if any providing provider is installed
        const missingProviders = providerIds.filter(pid => {
          const prov = providers.find(p => p.id === pid);
          return !prov?.installed || !prov?.running;
        });

        if (missingProviders.length > 0) {
          return {
            atomKey, atomLabel,
            status: 'missing-provider' as const,
            providers: providerIds,
            missingProviders,
          };
        }

        return {
          atomKey, atomLabel,
          status: 'ok' as const,
          providers: providerIds,
        };
      });

      // Determine overall status
      let overall: DiagStatus;
      const hasNoProvider = atomDiags.some(a => a.status === 'no-provider');
      const hasMissing = atomDiags.some(a => a.status === 'missing-provider');
      const allOptional = atomDiags.every(a => a.status === 'optional');
      const allOk = atomDiags.every(a => a.status === 'ok' || a.status === 'optional');

      if (hasNoProvider) overall = 'unsupported';
      else if (hasMissing) overall = 'missing-provider';
      else if (allOptional) overall = 'optional-only';
      else if (allOk) overall = 'available';
      else overall = 'degraded';

      // Collect all missing provider names
      const mpNames = new Set<string>();
      atomDiags.forEach(a => {
        (a.missingProviders || []).forEach(pid => {
          const prov = providers.find(p => p.id === pid);
          mpNames.add(prov?.name || pid);
        });
      });

      // Generate fix suggestion
      let fix = '';
      if (overall === 'unsupported') {
        const missingAtoms = atomDiags.filter(a => a.status === 'no-provider').map(a => a.atomLabel);
        fix = `原子 ${missingAtoms.join('、')} 无 Provider 实现，需扩展 Provider 或切换模式`;
      } else if (overall === 'missing-provider') {
        fix = `安装 ${[...mpNames].join('、')}`;
      } else if (overall === 'optional-only') {
        fix = '所有原子均为可选，不影响 Apply';
      }

      return {
        composition: comp,
        overall,
        atoms: atomDiags,
        missingProviderNames: [...mpNames],
        fix,
      };
    });
  }, [compositions, providerRows, providers]);

  // Don't render if no compositions
  if (!compositions.length) return null;

  return (
    <div className="border border-a-border/30 rounded-a-md overflow-hidden">
      <div className="px-3 py-2 bg-a-bg/30 border-b border-a-border/30">
        <span className="text-[10px] text-a-muted uppercase tracking-wider">组合能力诊断</span>
        <span className="ml-2 text-[10px]">
          {(() => {
            const avail = compositions.filter(c => c.status === 'available').length;
            const miss = compositions.filter(c => c.status === 'missing_provider').length;
            const unsup = compositions.filter(c => c.status === 'unsupported').length;
            const parts = [];
            if (avail > 0) parts.push(<span key="ok" className="text-[#4cd964]">{avail} 可用</span>);
            if (miss > 0) parts.push(<span key="miss" className="text-[#ff5c72]">{miss} 缺Provider</span>);
            if (unsup > 0) parts.push(<span key="unsup" className="text-a-muted/50">{unsup} 不支持</span>);
            return parts.length > 0 ? parts.reduce((a,b) => <>{a} <span className="text-a-border/50">·</span> {b}</>) : <span className="text-a-muted/50">—</span>;
          })()}
        </span>
      </div>
      <table className="w-full text-[10px]">
        <thead>
          <tr className="border-b border-a-border/20 bg-a-bg/20 text-a-muted">
            <th className="text-left py-1.5 px-3 font-medium w-[120px]">组合能力</th>
            <th className="text-left py-1.5 px-3 font-medium w-[72px]">状态</th>
            <th className="text-left py-1.5 px-3 font-medium">原子链</th>
            <th className="text-left py-1.5 px-3 font-medium">阻塞原因</th>
            <th className="text-left py-1.5 px-3 font-medium">修复</th>
          </tr>
        </thead>
        <tbody>
          {diags.map(d => {
            const st = DIAG_STATUS_STYLE[d.overall];
            return (
              <tr key={d.composition.name} className="border-b border-a-border/20 hover:bg-a-border/5 transition-colors">
                <td className="py-1.5 px-3">
                  <span className={cn('font-medium', d.overall === 'unsupported' ? 'text-a-muted/50' : 'text-a-fg')}>
                    {d.composition.name}
                  </span>
                </td>
                <td className="py-1.5 px-3">
                  <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium', st.badge)}>
                    {st.label}
                  </span>
                </td>
                <td className="py-1.5 px-3">
                  <div className="flex items-center gap-0.5 flex-wrap font-mono">
                    {d.atoms.length === 0 ? (
                      <span className="text-a-muted/40">—</span>
                    ) : (
                      d.atoms.map((a, i) => (
                        <span key={a.atomKey} className="flex items-center gap-0.5">
                          {i > 0 && <span className="text-a-border/50">→</span>}
                          <span className={cn(
                            a.status === 'ok' && 'text-[#4cd964]',
                            a.status === 'missing-provider' && 'text-[#ff5c72]',
                            a.status === 'no-provider' && 'text-a-muted/40 line-through',
                            a.status === 'optional' && 'text-a-border',
                          )}>
                            {a.atomLabel}
                            {a.status === 'ok' && ' ✓'}
                            {a.status === 'missing-provider' && ' ✗'}
                            {a.status === 'no-provider' && ' ✗'}
                            {a.status === 'optional' && ' ○'}
                          </span>
                        </span>
                      ))
                    )}
                  </div>
                </td>
                <td className="py-1.5 px-3">
                  {d.overall === 'unsupported' && (
                    <span className="text-a-muted/60">
                      {d.atoms.filter(a => a.status === 'no-provider').map(a => a.atomLabel).join('、')} 无 Provider
                    </span>
                  )}
                  {d.overall === 'missing-provider' && (
                    <span className="text-[#ff5c72]/80">
                      {d.missingProviderNames.join('、')} 未安装
                    </span>
                  )}
                  {d.overall === 'available' && <span className="text-a-muted/40">—</span>}
                  {d.overall === 'optional-only' && <span className="text-a-muted/60">全部可选原子</span>}
                  {d.overall === 'degraded' && <span className="text-[#e8b830]/80">部分降级</span>}
                </td>
                <td className="py-1.5 px-3">
                  <span className={cn(
                    d.overall === 'missing-provider' && 'text-[#ff5c72]',
                    d.overall === 'unsupported' && 'text-a-muted/60',
                    d.overall === 'available' && 'text-a-muted/40',
                    d.overall === 'optional-only' && 'text-a-muted/60',
                  )}>
                    {d.fix || '—'}
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function BindingMatrix({ mode, providers, onCellClick }: {
  mode: RuntimeModeDef;
  providers: ProviderState[];
  onCellClick: (ev: EvaluatedCell) => void;
}) {
  const atoms = mode.atoms || [];
  const providerRows = mode.providers || [];
  const compositions = mode.compositions || [];

  // Track which atom columns to highlight (from composition hover)
  const [highlightAtoms, setHighlightAtoms] = useState<Set<string>>(new Set());

  // Group atoms by layer for column headers
  const layerGroups = useMemo(() => {
    const groups: { layer: string; atoms: RuntimeAtom[] }[] = [];
    const seen = new Set<string>();
    for (const a of atoms) {
      if (!seen.has(a.layer)) {
        seen.add(a.layer);
        groups.push({ layer: a.layer, atoms: atoms.filter(x => x.layer === a.layer) });
      }
    }
    return groups;
  }, [atoms]);

  const matrix = useMemo(() => {
    const rows: { providerId: string; providerName: string; provider: ProviderState | undefined; cells: EvaluatedCell[] }[] = [];
    for (const prow of providerRows) {
      const prov = providers.find(p => p.id === prow.provider_id);
      const cells = atoms.map(atom => {
        const slots = prow.bindings?.[atom.key];
        return evaluateAtomSlots(slots, prov, prow.provider_id, atom);
      });
      rows.push({ providerId: prow.provider_id, providerName: prov?.name || prow.provider_id, provider: prov, cells });
    }
    return rows;
  }, [atoms, providerRows, providers]);

  // Compute mode issues
  const issues = useMemo(() => {
    const missing: EvaluatedCell[] = [];
    const conflicts: EvaluatedCell[] = [];
    for (const row of matrix) {
      for (const c of row.cells) {
        if (c.status === 'missing') missing.push(c);
        if (c.status === 'conflict') conflicts.push(c);
      }
    }
    return { missing, conflicts, canApply: missing.length === 0 && conflicts.length === 0 };
  }, [matrix]);

  return (
    <>
      {/* Composition cards */}
      <CompositionBar compositions={compositions} activeAtoms={highlightAtoms} onHover={(atoms) => setHighlightAtoms(atoms ? new Set(atoms) : new Set())} />

      {/* Composition diagnostic table */}
      <CompositionDiagnostic compositions={compositions} providerRows={providerRows} providers={providers} atoms={atoms} />

      {/* Status bar */}
      <div className={cn(
        'px-4 py-2.5 rounded-a-sm border text-xs flex items-center gap-3',
        issues.canApply ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15',
      )}>
        <span className={cn('font-semibold', issues.canApply ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
          {mode.label} Mode
        </span>
        <span className="text-a-muted">{mode.description}</span>
        <span className="flex-1" />
        {issues.canApply ? (
          <span className="text-[#4cd964] text-[11px] font-medium">✓ 可 Apply</span>
        ) : (
          <div className="flex items-center gap-3 text-[11px]">
            {issues.missing.length > 0 && (
              <span className="text-[#ff5c72]">{issues.missing.length} 个必需原子缺失</span>
            )}
            {issues.conflicts.length > 0 && (
              <span className="text-[#e8b830]">{issues.conflicts.length} 个端口冲突</span>
            )}
            <span className="text-[#ff5c72]/60">Cannot Apply</span>
          </div>
        )}
      </div>

      {/* Matrix table */}
      <div className="overflow-x-auto border border-a-border/30 rounded-a-md">
        <table className="w-full text-xs">
          {/* Two-tier header: layer groups → atom labels */}
          <thead>
            <tr className="border-b border-a-border/30 bg-a-bg/30">
              <th className="text-left py-2.5 px-3 font-medium text-a-muted w-28 border-r border-a-border/40" rowSpan={2}></th>
              {layerGroups.map((g, gi) => (
                <th key={g.layer} colSpan={g.atoms.length} className={cn(
                  'text-center py-1 px-2 text-[10px] font-medium border-b border-a-border/20',
                  gi < layerGroups.length - 1 && 'border-r border-a-border/30',
                  LAYER_HEADER[g.layer]?.color || 'text-a-muted',
                )}>
                  {LAYER_HEADER[g.layer]?.label || g.layer}
                </th>
              ))}
            </tr>
            <tr className="border-b border-a-border/30 bg-a-bg/20">
              {atoms.map((a, ai) => (
                <th key={a.key} className={cn(
                  'text-left py-2 px-3 font-medium text-a-muted border-r border-a-border/40',
                  highlightAtoms.has(a.key) && 'bg-a-accent/10 text-a-accent',
                )}>
                  <span className="text-[11px]">{a.label}</span>
                  <span className="ml-1 text-[9px] text-a-muted/50">{a.key}</span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {matrix.map(row => (
              <tr key={row.providerId} className="border-b border-a-border/20">
                <td className="py-2 px-3 border-r border-a-border/40">
                  <div className="flex items-center gap-2">
                    <HealthDot status={row.provider?.running ? 'healthy' : row.provider?.installed ? 'degraded' : 'failed'} />
                    <span className={cn('font-semibold', !row.provider?.running && !row.provider?.installed ? 'text-[#ff5c72]' : 'text-a-fg')}>
                      {row.providerName}
                    </span>
                  </div>
                </td>
                {row.cells.map((ev, ci) => {
                  const st = STATUS_STYLES[ev.status];
                  return (
                    <td key={ci}
                      onClick={() => ev.slots.length > 0 && onCellClick(ev)}
                      className={cn(
                        'py-2 px-3 align-top transition-colors hover:brightness-110 border-r border-a-border/40',
                        st.cell,
                        highlightAtoms.has(atoms[ci]?.key) && 'ring-1 ring-a-accent/30 bg-a-accent/5',
                        ev.slots.length > 0 ? 'cursor-pointer' : '',
                      )}>
                      <CellContent ev={ev} />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 text-[10px] text-a-muted">
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-[#4cd964]" /> 活跃</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-[#ff5c72]" /> MISSING</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-[#e8b830]" /> CONFLICT</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-sm bg-a-border" /> OPTIONAL</span>
        <span className="flex items-center gap-1"><span className="text-a-muted/30">—</span> 不参与</span>
        <span className="text-a-muted/50">|</span>
        <span className="flex items-center gap-1"><span className="text-a-accent/40">◉</span> 组合流高亮</span>
      </div>

      {/* Missing provider banner */}
      {issues.missing.length > 0 && (
        <div className="p-3 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/15 space-y-2">
          <div className="text-[11px] font-semibold text-[#ff5c72]">模式不可 Apply — 必需原子缺失</div>
          {issues.missing.map((c, i) => (
            <div key={i} className="flex items-center gap-3 text-[11px]">
              <span className="text-[#ff5c72] font-medium">{c.providerName}</span>
              <span className="text-a-muted">{c.atom.label}（{c.atom.key}）缺失</span>
              <span className="text-[#ff5c72]/60">{c.impact}</span>
            </div>
          ))}
          <div className="flex gap-2 pt-1">
            {issues.missing.some(c => c.providerId === 'haproxy') && (
              <Btn onClick={() => providerApi.install('haproxy').then(() => window.location.reload()).catch(() => {})}
                className="text-[10px]" primary>
                Install HAProxy
              </Btn>
            )}
            <Btn onClick={() => {}} className="text-[10px]">Switch to Legacy</Btn>
          </div>
        </div>
      )}
    </>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Cell Detail Drawer
// ══════════════════════════════════════════════════════════════════════════════

function CellDrawer({ cell, open, onClose }: { cell: EvaluatedCell | null; open: boolean; onClose: () => void }) {
  if (!cell) return null;
  const st = STATUS_STYLES[cell.status];

  return (
    <Drawer open={open} onClose={onClose} title={`Atom: ${cell.atom.label}`} subtitle={`${cell.providerName} / ${cell.atom.layer} / ${cell.atom.key}`} width="sm">
      <div className="space-y-4">
        {cell.slots.length > 0 && (
          <Card title={`Slots (${cell.slots.length})`}>
            {cell.slots.map((s, i) => (
              <div key={i} className={cn('py-2', i > 0 && 'border-t border-a-border/20')}>
                {s.port > 0 ? (
                  <div className="flex items-center gap-1.5 text-sm font-mono flex-wrap">
                    <span className="text-a-muted">{s.listening_at || `:${s.port}/${s.protocol}`}</span>
                    <span className="text-a-border">→</span>
                    <span className="text-a-accent font-semibold">{s.action}</span>
                    <span className="text-a-border">→</span>
                    <span className="text-a-fg2">{s.target}</span>
                  </div>
                ) : (
                  <div className="flex items-center gap-1.5 text-sm font-mono flex-wrap">
                    <span className="text-a-accent font-semibold">{s.action}</span>
                    <span className="text-a-border">→</span>
                    <span className="text-a-fg2">{s.target}</span>
                  </div>
                )}
                {s.note && <div className="text-[10px] text-a-muted mt-1">Note: {s.note}</div>}
                <div className="text-[10px] text-a-muted mt-1">
                  Required: {s.required ? '是' : '否（可选）'}
                  {s.purpose && <span className="ml-2">Purpose: {s.purpose}</span>}
                </div>
              </div>
            ))}
          </Card>
        )}

        <Card title="Status">
          <div className="flex items-center gap-2">
            <span className={cn('w-2.5 h-2.5 rounded-full', st.dot)} />
            <span className={cn('font-semibold text-sm', cell.status === 'missing' ? 'text-[#ff5c72]' : 'text-a-fg')}>
              {statusLabel(cell.status)}
            </span>
          </div>
          {cell.problem && (
            <div className="mt-2 p-3 rounded bg-[#ff5c72]/5 border border-[#ff5c72]/10 text-xs text-[#ff5c72]">
              {cell.problem}
            </div>
          )}
          {cell.impact && (
            <div className="text-xs text-a-fg2 mt-2">
              <span className="text-a-muted">影响: </span>{cell.impact}
            </div>
          )}
        </Card>

        {/* Actions */}
        <div className="pt-2 border-t border-a-border/30">
          <span className="text-[10px] text-a-muted uppercase tracking-wider block mb-2">Actions</span>
          <div className="flex flex-wrap gap-2">
            {cell.status === 'missing' && cell.providerId === 'haproxy' && (
              <Btn primary onClick={() => { providerApi.install('haproxy').then(() => onClose()).catch(() => {}); }} className="text-[10px]">
                Install HAProxy
              </Btn>
            )}
            {cell.status === 'conflict' && (
              <Btn onClick={() => {}} className="text-[10px]">Resolve Conflict</Btn>
            )}
            {cell.status === 'missing' && (
              <Btn onClick={() => {}} className="text-[10px]">
                {cell.providerId === 'haproxy' ? 'Use Nginx fallback' : 'Switch Mode'}
              </Btn>
            )}
          </div>
        </div>
      </div>
    </Drawer>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Provider detail cards (collapsed into each provider row's expanded view)
// ══════════════════════════════════════════════════════════════════════════════

function getCapStatus(prov: ProviderState, capKey: string): 'native' | 'theoretical' | 'unsupported' {
  if (prov.capabilities?.includes(capKey)) return 'native';
  if (prov.theoretical_capabilities?.includes(capKey)) return 'theoretical';
  return 'unsupported';
}

function CapBar({ native, theoretical, unsupported, total }: {
  native: number; theoretical: number; unsupported: number; total: number;
}) {
  const pct = (n: number) => total > 0 ? (n / total * 100).toFixed(1) : '0';
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-full bg-a-border/20 overflow-hidden flex">
        {native > 0 && <div className="h-full bg-[#4cd964]" style={{ width: `${pct(native)}%` }} />}
        {theoretical > 0 && <div className="h-full bg-[#e8b830]" style={{ width: `${pct(theoretical)}%` }} />}
      </div>
      <span className="text-[10px] font-mono text-a-muted w-8 text-right">{native}/{total}</span>
    </div>
  );
}

function ProviderCard({ provider, universe }: { provider: ProviderState; universe: CapabilityDef[] }) {
  const [expanded, setExpanded] = useState(false);
  const toast = useToast();
  const isAvailable = provider.installed && provider.running;
  const nativeCount = provider.capabilities?.length || 0;
  const theoCount = provider.theoretical_capabilities?.filter(c => !provider.capabilities?.includes(c)).length || 0;
  const unsupCount = universe.length - nativeCount - theoCount;

  const layerOrder = ['L3', 'L4', 'L5', 'L6', 'L7'];
  const byLayer = new Map<string, CapabilityDef[]>();
  for (const c of universe) {
    const list = byLayer.get(c.layer) || [];
    list.push(c); byLayer.set(c.layer, list);
  }

  const LAYER_COLORS: Record<string, string> = {
    L3: 'border-l-purple-500/40 bg-purple-500/3', L4: 'border-l-blue-500/40 bg-blue-500/3',
    L5: 'border-l-teal-500/40 bg-teal-500/3', L6: 'border-l-amber-500/40 bg-amber-500/3',
    L7: 'border-l-green-500/40 bg-green-500/3',
  };

  return (
    <div className={cn('border rounded-a-md transition-all',
      isAvailable ? 'bg-a-surface border-a-border'
        : provider.installed ? 'bg-a-surface/70 border-[#e8b830]/20'
          : 'bg-[#ff5c72]/3 border-[#ff5c72]/15',
    )}>
      <button onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 text-left cursor-pointer hover:bg-a-border/5 transition-colors">
        <HealthDot status={provider.running ? 'healthy' : provider.installed ? 'degraded' : 'failed'} />
        <span className={cn('font-semibold text-sm', isAvailable ? 'text-a-fg' : 'text-[#ff5c72]')}>{provider.name}</span>
        <StatusBadge status={provider.status} />
        <span className={cn('text-[11px] font-mono ml-auto mr-2', isAvailable ? 'text-a-muted' : 'text-[#ff5c72]/60')}>
          {nativeCount}/{universe.length}
        </span>
        <svg className={cn('w-4 h-4 text-a-muted transition-transform shrink-0', expanded && 'rotate-180')}
          viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M6 9l6 6 6-6"/></svg>
      </button>
      {expanded && (
        <div className="px-4 pb-4 border-t border-a-border/30 pt-3 space-y-4">
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
            <div><span className="text-a-muted">Gateway Type</span><div className="text-a-fg font-mono mt-0.5 font-medium">{provider.gateway_type}</div></div>
            <div><span className="text-a-muted">Binary</span><div className={cn('font-mono mt-0.5 truncate', provider.binary_path ? 'text-a-fg' : 'text-[#ff5c72]/60')}>{provider.binary_path || '未检测到'}</div></div>
            <div><span className="text-a-muted">Config</span><div className="text-a-fg font-mono mt-0.5 truncate">{provider.config_path || '—'}</div></div>
            <div><span className="text-a-muted">Ports</span><div className="text-a-fg font-mono mt-0.5">{provider.ports?.length ? provider.ports.map(b => `${b.port}/${b.protocol}(${b.purpose})`).join(', ') : '—'}</div></div>
          </div>
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <span className="text-[10px] text-a-muted uppercase tracking-wider">Capabilities</span>
              <span className={cn('text-[10px] font-mono', isAvailable ? 'text-a-muted' : 'text-[#ff5c72]/60')}>{nativeCount}/{universe.length}</span>
            </div>
            <CapBar native={nativeCount} theoretical={theoCount} unsupported={unsupCount} total={universe.length} />
            <div className="flex gap-3 mt-1 text-[9px]"><span className="text-[#4cd964]">{nativeCount} 原生</span><span className="text-[#e8b830]">{theoCount} 可实现</span><span className="text-a-muted">{unsupCount} 不适用</span></div>
          </div>
          <div className="space-y-1.5">
            {layerOrder.map(layer => {
              const caps = byLayer.get(layer);
              if (!caps?.length) return null;
              return (
                <div key={layer} className="flex items-start gap-2 text-[10px]">
                  <span className={cn('px-1 py-0.5 rounded font-mono w-8 text-center shrink-0 mt-0.5', LAYER_COLORS[layer])}>{layer}</span>
                  <div className="flex flex-wrap gap-1 flex-1">
                    {caps.map(cap => {
                      const s = getCapStatus(provider, cap.key);
                      return (
                        <span key={cap.key} title={`${cap.label}: ${cap.description}`}
                          className={cn('px-1.5 py-0.5 rounded text-[10px] font-mono',
                            s === 'native' ? 'bg-[#4cd964]/10 text-[#4cd964] border border-[#4cd964]/20' :
                            s === 'theoretical' ? 'bg-[#e8b830]/10 text-[#e8b830] border border-[#e8b830]/20' :
                            'bg-a-border/10 text-a-muted/50 border border-a-border/10')}>
                          {s === 'native' ? '✓' : s === 'theoretical' ? '△' : '—'} {cap.key}
                        </span>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
          {provider.installed ? (
            <div className="flex gap-2 pt-1 border-t border-a-border/30">
              <Btn onClick={() => { providerApi.reload(provider.id).then(() => toast(`${provider.name} 重载成功`)).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">热重载</Btn>
              <Btn onClick={() => { providerApi.getConfig(provider.id).then(c => toast(JSON.stringify(c))).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">查看配置</Btn>
              <Btn onClick={() => { providerApi.diagnoseAll().then(() => toast('诊断完成')).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">诊断</Btn>
            </div>
          ) : (
            <div className="flex gap-2 pt-1 border-t border-a-border/30">
              <Btn primary onClick={() => { providerApi.install(provider.id).then(() => toast(`${provider.name} 安装成功`)).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">安装</Btn>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Main Page
// ══════════════════════════════════════════════════════════════════════════════

const PAGE_TABS = [
  { key: 'binding', label: 'Runtime Binding' },
  { key: 'matrix', label: '能力矩阵' },
  { key: 'detail', label: 'Provider 详情' },
];

export default function Providers() {
  const [mode, setMode] = useState('legacy');
  const [pageTab, setPageTab] = useState('binding');
  const [drawerCell, setDrawerCell] = useState<EvaluatedCell | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data: provData, isLoading } = useQuery({
    queryKey: ['providers'],
    queryFn: async () => {
      const result = await (providerApi as any).list();
      return {
        providers: (result?.providers || []) as ProviderState[],
        universe: (result?.capability_universe || []) as CapabilityDef[],
      };
    },
  });

  // v1.8L-20: Runtime mode from API replaces hand-coded MODES[] and port-policy query
  const { data: runtimeMode } = useQuery({
    queryKey: ['runtime-mode'],
    queryFn: () => runtimeModeApi.get(),
    refetchInterval: 120_000,
  });

  const providers = provData?.providers || [];
  const universe = provData?.universe || [];

  const availableModes: RuntimeModeDef[] = runtimeMode?.available_modes || [];
  const currentMode = availableModes.find(m => m.id === mode)
    || runtimeMode?.current
    || availableModes[0];

  // Sync mode from backend detection on first load
  const effectiveModeId = runtimeMode?.current?.id || 'legacy';

  const handleCellClick = (ev: EvaluatedCell) => {
    setDrawerCell(ev);
    setDrawerOpen(true);
  };

  // Initialize mode from backend on first load
  if (mode === 'legacy' && effectiveModeId !== 'legacy' && availableModes.length > 0) {
    // Defer state update to avoid render-loop
    queueMicrotask(() => setMode(effectiveModeId));
  }

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="网关运行时矩阵"
        subtitle={`${providers.length} 个 Provider · 当前模式: ${runtimeMode?.current?.label || 'Legacy'} · 可用模式: ${availableModes.filter(m => m.implemented).length}`}
      />

      {/* Page tabs: Binding Matrix | Capability Matrix | Provider Detail */}
      <TabBar tabs={PAGE_TABS} active={pageTab} onChange={setPageTab} />

      {pageTab === 'binding' && currentMode && (
        <>
          {/* Mode switcher */}
          <div>
            <ModeSwitcher active={mode} onChange={setMode} modes={availableModes} />
            {effectiveModeId !== mode && availableModes.find(m => m.id === mode)?.implemented && (
              <div className="mt-2 text-[11px] text-[#e8b830]">
                当前运行模式是 {runtimeMode?.current?.label || effectiveModeId}，你正在预览 {currentMode.label} 模式的绑定矩阵
              </div>
            )}
          </div>

          {/* Binding matrix */}
          <BindingMatrix mode={currentMode} providers={providers} onCellClick={handleCellClick} />
        </>
      )}

      {pageTab === 'matrix' && (
        isLoading ? (
          <Card title="能力矩阵"><div className="text-sm text-a-muted py-12 text-center">加载中...</div></Card>
        ) : (
          <CapabilityMatrixTab providers={providers} universe={universe} />
        )
      )}

      {pageTab === 'detail' && (
        isLoading ? (
          <Card title="Provider 详情"><div className="text-sm text-a-muted py-12 text-center">加载中...</div></Card>
        ) : providers.length > 0 ? (
          <Card title="Provider 详情" subtitle="点击展开查看完整能力清单、诊断信息和操作">
            <div className="space-y-2">
              {providers.map(p => <ProviderCard key={p.id} provider={p} universe={universe} />)}
            </div>
          </Card>
        ) : (
          <Card title="Provider 详情">
            <div className="text-center py-12 text-sm text-a-muted">
              没有检测到 Provider · 运行 <code className="text-a-accent">aegis doctor</code> 诊断
            </div>
          </Card>
        )
      )}

      {/* Cell detail drawer */}
      <CellDrawer cell={drawerCell} open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════════════════
// Capability Matrix Tab (reused from previous version, condensed)
// ══════════════════════════════════════════════════════════════════════════════

function CapabilityMatrixTab({ providers, universe }: { providers: ProviderState[]; universe: CapabilityDef[] }) {
  if (!universe.length || !providers.length) {
    return <Card title="能力矩阵"><div className="text-xs text-a-muted py-8 text-center">数据不可用</div></Card>;
  }

  const layers = new Map<string, CapabilityDef[]>();
  for (const c of universe) {
    const list = layers.get(c.layer) || [];
    list.push(c); layers.set(c.layer, list);
  }
  const layerOrder = ['L3', 'L4', 'L5', 'L6', 'L7'];

  const stats = providers.map(p => ({
    id: p.id, name: p.name, installed: p.installed,
    native: universe.filter(c => p.capabilities?.includes(c.key)).length,
    theoretical: universe.filter(c => !p.capabilities?.includes(c.key) && p.theoretical_capabilities?.includes(c.key)).length,
    total: universe.length,
  }));

  return (
    <Card title="能力矩阵" subtitle={`${universe.length} 项能力 × ${providers.length} 个 Provider`}>
      {/* Provider count bars */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
        {stats.map(s => (
          <div key={s.id} className={cn('p-3 rounded-a-sm border',
            s.installed ? 'bg-a-surface border-a-border' : 'bg-[#ff5c72]/3 border-[#ff5c72]/20')}>
            <div className="flex items-center gap-2 mb-1.5">
              <HealthDot status={s.installed ? 'healthy' : 'failed'} />
              <span className={cn('text-xs font-semibold', s.installed ? 'text-a-fg' : 'text-[#ff5c72]')}>{s.name}</span>
              <span className={cn('text-[10px] font-mono ml-auto', s.installed ? 'text-a-muted' : 'text-[#ff5c72]/60')}>{s.native}/{s.total}</span>
            </div>
            <CapBar native={s.native} theoretical={s.theoretical} unsupported={s.total - s.native - s.theoretical} total={s.total} />
            <div className="flex gap-3 mt-1.5 text-[9px]">
              <span className="text-[#4cd964]">{s.native} 原生</span>
              <span className="text-[#e8b830]">{s.theoretical} 可实现</span>
              <span className="text-a-muted">{s.total - s.native - s.theoretical} 不适用</span>
            </div>
          </div>
        ))}
      </div>

      <div className="flex items-center gap-4 mb-3 text-[10px]">
        <span className="flex items-center gap-1"><span className="text-[#4cd964] font-bold">✓</span> 原生支持</span>
        <span className="flex items-center gap-1"><span className="text-[#e8b830] font-bold">△</span> 可实现</span>
        <span className="flex items-center gap-1"><span className="text-a-muted">—</span> 不适用</span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-xs border-collapse">
          <thead>
            <tr className="border-b-2 border-a-border/50">
              <th className="text-left py-2 px-2 font-medium text-a-muted w-10">层</th>
              <th className="text-left py-2 px-2 font-medium text-a-muted">能力</th>
              {providers.map(p => {
                const st = stats.find(s => s.id === p.id)!;
                return <th key={p.id} className={cn('text-center py-2 px-1 min-w-[58px]', !p.installed && 'text-[#ff5c72]/60')}>
                  <div className="text-[10px] font-medium">{p.name}</div>
                  <div className={cn('text-[10px] font-mono', p.installed ? 'text-a-muted' : 'text-[#ff5c72]/50')}>{st.native}/{st.total}</div>
                </th>;
              })}
            </tr>
          </thead>
          <tbody>
            {layerOrder.map(layer => {
              const caps = layers.get(layer);
              if (!caps?.length) return null;
              const LAYER_COLORS: Record<string, string> = {
                L3: 'border-l-purple-500/40 bg-purple-500/3', L4: 'border-l-blue-500/40 bg-blue-500/3',
                L5: 'border-l-teal-500/40 bg-teal-500/3', L6: 'border-l-amber-500/40 bg-amber-500/3',
                L7: 'border-l-green-500/40 bg-green-500/3',
              };
              return caps.map((cap, ci) => (
                <tr key={cap.key} className={cn('border-b border-a-border/20 hover:bg-a-border/5', ci === 0 && 'border-t border-a-border/30')}>
                  {ci === 0 && <td className="py-2 px-2 text-[10px] text-a-muted font-medium align-top" rowSpan={caps.length}>
                    <span className={cn('px-1 py-0.5 rounded', LAYER_COLORS[layer])}>{layer}</span></td>}
                  <td className="py-2 px-2"><div className="text-a-fg text-[11px]">{cap.label}</div><div className="text-[10px] text-a-muted">{cap.key}</div></td>
                  {providers.map(p => {
                    const s = getCapStatus(p, cap.key);
                    return <td key={p.id} className={cn('text-center py-2 px-1', !p.installed && 'opacity-50')}>
                      {s === 'native' && <span className="text-[#4cd964] font-bold text-sm">✓</span>}
                      {s === 'theoretical' && <span className="text-[#e8b830] font-bold text-sm">△</span>}
                      {s === 'unsupported' && <span className="text-a-muted text-sm">—</span>}
                    </td>;
                  })}
                </tr>
              ));
            })}
          </tbody>
        </table>
      </div>

      <div className="mt-4 p-3 rounded bg-a-border/5 border border-a-border/20">
        <span className="text-[10px] text-a-muted uppercase tracking-wider">当前节点可支持能力</span>
        <div className="text-[11px] text-a-fg2 mt-1">
          {(() => {
            const union = new Set<string>();
            providers.filter(p => p.installed).forEach(p => p.capabilities?.forEach(c => union.add(c)));
            return `${union.size} 项能力可用（${providers.filter(p => p.installed).map(p => p.name).join(' + ') || '无'} 能力并集）`;
          })()}
        </div>
      </div>
    </Card>
  );
}
