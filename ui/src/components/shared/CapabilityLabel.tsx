import { getCapMeta } from '@/lib/capability-labels';
import { cn } from '@/lib/utils';

interface Props {
  cap: string;
  size?: 'sm' | 'md';
  showLabel?: boolean;
  className?: string;
}

const svgMap: Record<string, JSX.Element> = {
  auto_cert:       <path d="M8 2a5 5 0 0 1 5 5v2h1a1 1 0 0 1 0 2h-1v1a1 1 0 0 1-2 0v-1H4a1 1 0 0 1-1-1V7a5 5 0 0 1 5-5zm0 2a3 3 0 0 0-3 3v2h6V7a3 3 0 0 0-3-3z" fill="currentColor"/>,
  load_cert:       <path d="M5 2a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V6l-3-3H5zm4 1v2h2v8H6V3h3zM7 9v1h2V9H7z" fill="currentColor"/>,
  tls_terminate:   <path d="M8 1a6 6 0 0 0-6 6v3h12V7a6 6 0 0 0-6-6zm-3 6a3 3 0 1 1 6 0H5zm-4 4v1a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-1H1z" fill="currentColor"/>,
  tls_passthrough: <path d="M2 4l4 4-4 4M14 4l-4 4 4 4M8 2v12" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round"/>,
  sni_preread:     <path d="M3 8h10M3 8l3-3M3 8l3 3M13 8l-3-3M13 8l-3 3" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/>,
  mtls_terminate:  <path d="M8 1a3 3 0 0 0-3 3v2H3l2 5h6l2-5h-2V4a3 3 0 0 0-3-3zm0 9a1 1 0 1 0 0 2 1 1 0 0 0 0-2z" fill="currentColor"/>,
  tls_masquerade:  <path d="M3 8l2 4h2l1-6H6L5 9 4 8l-1-2H1l2 4v2h2l2-6H5zM10 12l2-4 1 1 1 2h2l-2-4H12l-2 6h2z" fill="currentColor"/>,
  listen_tcp: (
    <g>
      <circle cx="8" cy="8" r="3" fill="currentColor"/><path d="M5 4v8M11 4v8" stroke="currentColor" strokeWidth="1.5" fill="none"/>
    </g>
  ),
  listen_udp: (
    <g>
      <circle cx="8" cy="8" r="3" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 2"/><path d="M5 4v8M11 4v8" stroke="currentColor" strokeWidth="1" strokeDasharray="2 2"/>
    </g>
  ),
  upstream_tcp:    <path d="M2 8l4-4v3h4v2H6v3zM10 12l4-4-4-4v3H6v2h4v3z" fill="currentColor"/>,
  upstream_udp:    <path d="M2 8l4-4v3h4v2H6v3zM10 12l4-4-4-4v3H6v2h4v3z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 2"/>,
  route_host:      <path d="M3 3h10v4H3zm0 6h10v4H3zM8 2v12" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round"/>,
  route_path:      <path d="M2 6l2 2-2 2M14 6l-2 2 2 2M8 6v4M5 4h6l-1 8H6z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>,
  hot_reload:      <path d="M8 2a6 6 0 0 0-5.1 3h1.3A4.8 4.8 0 0 1 8 3.2c1.7 0 3.1.9 3.9 2.2L9 7h5V2l-1.8 1.8A6 6 0 0 0 8 2zm0 12a6 6 0 0 0 5.1-3h-1.3A4.8 4.8 0 0 1 8 12.8c-1.7 0-3.1-.9-3.9-2.2L7 9H2v5l1.8-1.8A6 6 0 0 0 8 14z" fill="currentColor"/>,
  validate_cfg: (
    <g>
      <path d="M6 8l2 2 4-4" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round"/><rect x="2" y="2" width="12" height="12" rx="2" fill="none" stroke="currentColor" strokeWidth="1.5"/>
    </g>
  ),
  health_check:    <path d="M8 13l-4-4.5C2.5 7 2 5.5 2 4.5 2 3 3 2 4.5 2 5.7 2 6.8 2.6 8 4c1.2-1.4 2.3-2 3.5-2C13 2 14 3 14 4.5c0 1-.5 2.5-2 4L8 13z" fill="currentColor"/>,
  load_balance: (
    <g>
      <circle cx="3" cy="5" r="1.5" fill="currentColor"/><circle cx="3" cy="11" r="1.5" fill="currentColor"/><line x1="4.5" y1="5" x2="12" y2="5" stroke="currentColor" strokeWidth="1.5"/><line x1="4.5" y1="11" x2="12" y2="11" stroke="currentColor" strokeWidth="1.5"/>
    </g>
  ),
  rate_limit: (
    <g>
      <path d="M2 12h12M4 12V7a2 2 0 0 1 4 0v5M10 12V9a1 1 0 0 0-1-1" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/><circle cx="13" cy="4" r="2" fill="none" stroke="currentColor" strokeWidth="1.5"/>
    </g>
  ),
  http1:           <text x="3" y="13" fontSize="10" fontWeight="bold" fill="currentColor">1</text>,
  http2:           <text x="3" y="13" fontSize="10" fontWeight="bold" fill="currentColor">2</text>,
  http3:           <text x="3" y="13" fontSize="10" fontWeight="bold" fill="currentColor">3</text>,
  grpc:            <text x="1" y="13" fontSize="8" fontWeight="bold" fill="currentColor">gR</text>,
  ws:              <path d="M2 4l4 4-4 4M14 4l-4 4 4 4" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round"/>,
  connect:         <path d="M2 5h4v6H2zM10 5h4v6h-4zM6 6h4v4H6z" fill="currentColor" fillOpacity="0.3" stroke="currentColor" strokeWidth="1.5"/>,
  alpn_match: (
    <g>
      <path d="M4 4h8M4 8h8M4 12h8" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/><circle cx="3" cy="4" r="1" fill="currentColor"/><circle cx="3" cy="8" r="1" fill="currentColor"/><circle cx="3" cy="12" r="1" fill="currentColor"/><circle cx="13" cy="4" r="1" fill="currentColor"/><circle cx="13" cy="8" r="1" fill="currentColor"/><circle cx="13" cy="12" r="1" fill="currentColor"/>
    </g>
  ),
  ocsp_stapling: (
    <g>
      <path d="M3 9l2 2 4-4" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/><rect x="2" y="2" width="12" height="12" rx="1" fill="none" stroke="currentColor" strokeWidth="1.5"/><circle cx="8" cy="8" r="5" fill="none" stroke="currentColor" strokeWidth=".5" strokeDasharray="1 1"/>
    </g>
  ),
  tcp_splice: (
    <g>
      <path d="M3 5l3 3v4M13 5l-3 3v4" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/><path d="M3 12h10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/><path d="M6 2l2 3-2 3" fill="currentColor"/>
    </g>
  ),
  embedded: (
    <g>
      <rect x="5" y="2" width="6" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1.5"/><rect x="2" y="4" width="12" height="10" rx="2" fill="none" stroke="currentColor" strokeWidth="1.5"/><path d="M7 8h2M7 10h1" stroke="currentColor" strokeWidth="1" strokeLinecap="round"/>
    </g>
  ),
  config_read:     <path d="M8 2a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 2l-3 8h1.5l.7-2h1.6l.7 2H11L8 4zm-1.2 4.5L8 5.5l1.2 3H6.8z" fill="currentColor"/>,
  config_write:    <path d="M2 13l1-1h10l1 1v1H2zM10 2l3 3-6 6H4v-3z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>,
  can_install: (
    <g>
      <path d="M8 2v10M8 2L5 5M8 2l3 3" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/><rect x="2" y="12" width="12" height="2" rx="1" fill="currentColor"/>
    </g>
  ),
};

const fallback = <circle cx="8" cy="8" r="4" fill="none" stroke="currentColor" strokeWidth="1.5"/>;

export default function CapabilityLabel({ cap, size = 'sm', showLabel = true, className }: Props) {
  const meta = getCapMeta(cap);
  const icon = svgMap[cap] || fallback;
  const dim = size === 'sm' ? 'w-3 h-3' : 'w-4 h-4';
  const text = size === 'sm' ? 'text-[10px]' : 'text-xs';

  return (
    <span className={cn('inline-flex items-center gap-0.5 align-middle', className)}>
      <svg className={dim} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        {icon}
      </svg>
      {showLabel && <span className={cn(text, 'text-a-muted')}>{meta?.shortLabel || cap}</span>}
    </span>
  );
}

export function CapabilityBadge({ cap, className }: { cap: string; className?: string }) {
  const meta = getCapMeta(cap);
  return (
    <span className={cn(
      'inline-flex items-center gap-0.5 px-1 py-0.5 rounded border border-a-border/30 bg-a-surface text-[10px]',
      className
    )}>
      <CapabilityLabel cap={cap} showLabel={false} />
      <span>{meta?.shortLabel || cap}</span>
    </span>
  );
}
