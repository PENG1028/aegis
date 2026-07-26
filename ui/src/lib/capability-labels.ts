// Capability labels — single source of truth for capability display metadata.
// All labels, layers, and SVG icon paths live here.
// Consumed by CapabilityLabel component, TlsSettings, InfraManagement, and Providers pages.

export interface CapMeta {
  key: string;
  layer: string;
  label: string;
  shortLabel: string;
}

const CAPS: CapMeta[] = [
  { key: 'auto_cert',       layer: 'L7', label: '自动证书',   shortLabel: 'ACME' },
  { key: 'load_cert',       layer: 'L7', label: '加载证书',   shortLabel: '证书' },
  { key: 'tls_terminate',   layer: 'L5', label: 'TLS 终结',   shortLabel: 'TLS' },
  { key: 'tls_passthrough', layer: 'L5', label: 'TLS 直通',   shortLabel: '直通' },
  { key: 'sni_preread',     layer: 'L5', label: 'SNI 分流',   shortLabel: 'SNI' },
  { key: 'mtls_terminate',  layer: 'L5', label: 'mTLS 认证',  shortLabel: 'mTLS' },
  { key: 'tls_masquerade',  layer: 'L5', label: 'TLS 卸载',   shortLabel: '卸载' },
  { key: 'listen_tcp',      layer: 'L4', label: 'TCP 监听',   shortLabel: 'TCP' },
  { key: 'listen_udp',      layer: 'L4', label: 'UDP 监听',   shortLabel: 'UDP' },
  { key: 'upstream_tcp',    layer: 'L4', label: 'TCP 转发',   shortLabel: '转发' },
  { key: 'upstream_udp',    layer: 'L4', label: 'UDP 转发',   shortLabel: 'UDP转' },
  { key: 'route_host',      layer: 'L7', label: '域名路由',   shortLabel: '域名' },
  { key: 'route_path',      layer: 'L7', label: '路径路由',   shortLabel: '路径' },
  { key: 'hot_reload',      layer: '—',  label: '热重载',     shortLabel: '重载' },
  { key: 'validate_cfg',    layer: '—',  label: '配置校验',   shortLabel: '校验' },
  { key: 'health_check',    layer: 'L7', label: '健康检查',   shortLabel: '健康' },
  { key: 'load_balance',    layer: 'L7', label: '负载均衡',   shortLabel: '均衡' },
  { key: 'rate_limit',      layer: 'L7', label: '限流',       shortLabel: '限流' },
  { key: 'http1',           layer: 'L7', label: 'HTTP/1.1',   shortLabel: 'H1' },
  { key: 'http2',           layer: 'L7', label: 'HTTP/2',     shortLabel: 'H2' },
  { key: 'http3',           layer: 'L7', label: 'HTTP/3',     shortLabel: 'H3' },
  { key: 'grpc',            layer: 'L7', label: 'gRPC',       shortLabel: 'gRPC' },
  { key: 'ws',              layer: 'L7', label: 'WebSocket',  shortLabel: 'WS' },
  { key: 'connect',         layer: 'L7', label: 'CONNECT',    shortLabel: 'CON' },
  { key: 'alpn_match',      layer: 'L6', label: 'ALPN 匹配',  shortLabel: 'ALPN' },
  { key: 'ocsp_stapling',   layer: 'L6', label: 'OCSP 装订',  shortLabel: 'OCSP' },
  { key: 'tcp_splice',      layer: 'L4', label: 'TCP 拼接',   shortLabel: '拼接' },
  { key: 'embedded',        layer: '—',  label: '嵌入式',     shortLabel: '嵌入' },
  { key: 'config_read',     layer: '—',  label: '配置读取',   shortLabel: '读取' },
  { key: 'config_write',    layer: '—',  label: '配置写入',   shortLabel: '写入' },
  { key: 'can_install',     layer: '—',  label: '可安装',     shortLabel: '安装' },
];

const capsByKey: Record<string, CapMeta> = {};
for (const c of CAPS) capsByKey[c.key] = c;

export function getCapMeta(key: string): CapMeta | undefined {
  return capsByKey[key];
}

export function getCapLabel(key: string): string {
  return capsByKey[key]?.label || key;
}

export const allCaps: CapMeta[] = CAPS;
