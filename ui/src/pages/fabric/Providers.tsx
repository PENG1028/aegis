// ─── Providers (Middleware) ───
// v1.8L: 3-dimension architecture — shows provider state, capabilities, diagnostics
import { useQuery } from '@tanstack/react-query';
import { providerApi } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge } from '@/components/shared';
import { cn } from '@/lib/utils';

type Capability = string;

interface ProviderState {
  id: string;
  name: string;
  gateway_type: string;
  status: string;
  installed: boolean;
  running: boolean;
  version: string;
  binary_path: string;
  config_path: string;
  capabilities: Capability[];
  ports: { port: number; owner: string; protocol: string; purpose: string; status: string }[];
  diagnostic?: {
    provider: string;
    installed: boolean;
    version: string;
    version_supported: boolean;
    config_valid?: boolean;
    service_running?: boolean;
    last_error_code: string;
    last_error_message: string;
  } | null;
}

function CapabilityBadge({ cap }: { cap: Capability }) {
  const layerMap: Record<string, { label: string; color: string }> = {
    route_src_ip: { label: 'L3', color: 'bg-purple-500/10 text-purple-400' },
    transparent_proxy: { label: 'L3', color: 'bg-purple-500/10 text-purple-400' },
    listen_tcp: { label: 'L4', color: 'bg-blue-500/10 text-blue-400' },
    listen_udp: { label: 'L4', color: 'bg-blue-500/10 text-blue-400' },
    upstream_tcp: { label: 'L4', color: 'bg-blue-500/10 text-blue-400' },
    upstream_udp: { label: 'L4', color: 'bg-blue-500/10 text-blue-400' },
    tls_terminate: { label: 'L5', color: 'bg-teal-500/10 text-teal-400' },
    tls_passthrough: { label: 'L5', color: 'bg-teal-500/10 text-teal-400' },
    mtls_terminate: { label: 'L5', color: 'bg-teal-500/10 text-teal-400' },
    tls_masquerade: { label: 'L5', color: 'bg-teal-500/10 text-teal-400' },
    sni_preread: { label: 'L6', color: 'bg-amber-500/10 text-amber-400' },
    alpn_match: { label: 'L6', color: 'bg-amber-500/10 text-amber-400' },
    proto_detect: { label: 'L6', color: 'bg-amber-500/10 text-amber-400' },
    ocsp_stapling: { label: 'L6', color: 'bg-amber-500/10 text-amber-400' },
  };

  const info = layerMap[cap] || { label: 'L7', color: 'bg-green-500/10 text-green-400' };
  return (
    <span className={`px-1.5 py-0.5 rounded text-[10px] font-mono ${info.color} flex items-center gap-1`}>
      <span className="opacity-50">{info.label}</span>
      {cap}
    </span>
  );
}

export default function Providers() {
  const { data, isLoading } = useQuery({
    queryKey: ['providers'],
    queryFn: async () => {
      const result = await (providerApi as any).list();
      return (result?.providers || result || []) as ProviderState[];
    },
  });

  const providers = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="中间件 Provider"
        subtitle={`${providers.length} 个 Provider · 三维度架构 (能力声明 / 拓扑组合 / 生命周期)`}
      />

      {isLoading ? (
        <div className="text-a-fg/50 text-sm">加载中...</div>
      ) : (
        <div className="space-y-3">
          {providers.map((p: ProviderState) => (
            <div
              key={p.id}
              className={cn(
                'p-4 rounded-a-md border transition-colors',
                p.running ? 'bg-a-surface border-a-border' : 'bg-a-surface/50 border-a-border/50',
              )}
            >
              {/* Header */}
              <div className="flex items-center gap-3 mb-3">
                <HealthDot status={p.running ? 'healthy' : p.installed ? 'degraded' : 'failed'} />
                <span className="font-semibold text-a-fg">{p.name}</span>
                <StatusBadge status={p.status} />
                <span className="text-xs text-a-fg/50 font-mono">{p.id}</span>
                {p.version && <span className="text-xs text-a-fg/30 ml-auto">v{p.version}</span>}
              </div>

              {/* Info row */}
              <div className="grid grid-cols-4 gap-3 mb-3 text-xs text-a-fg/60">
                <div>
                  <span className="text-a-fg/30">Gateway Type</span>
                  <div className="text-a-fg font-mono mt-0.5">{p.gateway_type}</div>
                </div>
                <div>
                  <span className="text-a-fg/30">Binary</span>
                  <div className="text-a-fg font-mono mt-0.5 truncate">{p.binary_path || '—'}</div>
                </div>
                <div>
                  <span className="text-a-fg/30">Config</span>
                  <div className="text-a-fg font-mono mt-0.5 truncate">{p.config_path || '—'}</div>
                </div>
                <div>
                  <span className="text-a-fg/30">Ports</span>
                  <div className="text-a-fg font-mono mt-0.5">
                    {p.ports?.map(port => `${port.port}/${port.protocol}(${port.purpose})`).join(', ') || '—'}
                  </div>
                </div>
              </div>

              {/* Capabilities (26 L3-L7 constants) */}
              <div className="mb-2">
                <span className="text-[10px] text-a-fg/30 uppercase tracking-wider">Capabilities</span>
                <div className="flex flex-wrap gap-1 mt-1">
                  {p.capabilities?.map(cap => (
                    <CapabilityBadge key={cap} cap={cap} />
                  ))}
                </div>
              </div>

              {/* Diagnostic (if available) */}
              {p.diagnostic && (
                <div className="border-t border-a-border/50 pt-2 mt-2">
                  <span className="text-[10px] text-a-fg/30 uppercase tracking-wider">Diagnostic</span>
                  <div className="grid grid-cols-4 gap-2 mt-1 text-[11px]">
                    <div>
                      <span className={p.diagnostic.installed ? 'text-green-400' : 'text-red-400'}>
                        {p.diagnostic.installed ? '✓' : '✗'} installed
                      </span>
                    </div>
                    <div>
                      <span className={p.diagnostic.version_supported ? 'text-green-400' : 'text-red-400'}>
                        {p.diagnostic.version_supported ? '✓' : '✗'} version
                      </span>
                    </div>
                    <div>
                      <span className={p.diagnostic.config_valid ? 'text-green-400' : 'text-red-400'}>
                        {p.diagnostic.config_valid ? '✓' : '✗'} config
                      </span>
                    </div>
                    <div>
                      <span className={p.diagnostic.service_running ? 'text-green-400' : 'text-red-400'}>
                        {p.diagnostic.service_running ? '✓' : '✗'} service
                      </span>
                    </div>
                  </div>
                  {p.diagnostic.last_error_code && (
                    <div className="text-[11px] text-red-400/70 mt-1">
                      {p.diagnostic.last_error_code}: {p.diagnostic.last_error_message}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}

          {providers.length === 0 && (
            <div className="text-center py-12 text-a-fg/40 text-sm">
              没有检测到 Provider · 检查中间件安装状态
            </div>
          )}
        </div>
      )}
    </div>
  );
}
