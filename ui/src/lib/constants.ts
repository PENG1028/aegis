// ─── Application Constants ───
// Workspace config, navigation items, and shared constants.

import type { ForwardingMode } from '@/types/workspace';

// ─── Workspace Definitions ───
export const WORKSPACES = [
  { id: 'command-center', path: '/', label: '总控台', icon: 'dashboard' },
  { id: 'exposure', path: '/exposure', label: '域名与路由', icon: 'exposure' },
  { id: 'fabric', path: '/fabric', label: '网关网络', icon: 'fabric' },
  { id: 'service-auth', path: '/auth', label: '服务认证', icon: 'shield' },
  { id: 'runtime', path: '/runtime', label: '节点运行时', icon: 'runtime' },
  { id: 'release', path: '/release', label: '配置发布', icon: 'release' },
  { id: 'observe', path: '/observe', label: '观测诊断', icon: 'observe' },
  { id: 'access', path: '/access', label: '访问控制', icon: 'access' },
  { id: 'settings', path: '/settings', label: '系统设置', icon: 'settings' },
] as const;

export type WorkspaceId = (typeof WORKSPACES)[number]['id'];

// ─── Workspace Sub-navigation ───
export const WORKSPACE_NAV: Record<string, { path: string; label: string }[]> = {
  exposure: [
    { path: '/exposure', label: '域名列表' },
    { path: '/exposure/new', label: '添加域名' },
  ],
  fabric: [
    { path: '/fabric', label: '能力矩阵' },
    { path: '/fabric/egress', label: '出口网关' },
    { path: '/fabric/mode', label: '运行时模式' },
    { path: '/fabric/service', label: '网关面板' },
  ],
  'service-auth': [
    { path: '/auth', label: '服务列表' },
    { path: '/auth/callgraph', label: '调用拓扑' },
  ],
  runtime: [
    { path: '/runtime', label: '节点' },
    { path: '/runtime/deploy', label: '部署节点' },
    { path: '/runtime/updates', label: '更新' },
    { path: '/runtime/sync', label: '同步状态' },
  ],
  release: [
    { path: '/release', label: '变更' },
    { path: '/release/diff', label: '差异' },
    { path: '/release/dry-run', label: '预演' },
    { path: '/release/apply', label: '推送' },
    { path: '/release/history', label: '历史' },
    { path: '/release/rollback', label: '回滚' },
  ],
  observe: [
    { path: '/observe', label: '链路追踪' },
    { path: '/observe/health', label: '健康检查' },
    { path: '/observe/safety', label: '安全检查' },
    { path: '/observe/logs', label: '操作日志' },
    { path: '/observe/audit', label: '审计日志' },
    { path: '/observe/doctor', label: '诊断' },
    { path: '/observe/acceptance', label: '验收' },
  ],
  access: [
    { path: '/access/admin', label: '管理员' },
    { path: '/access/credentials', label: '凭据' },
    { path: '/access/certificates', label: 'TLS 证书' },
    { path: '/access/tokens', label: '加入令牌' },
  ],
  settings: [
    { path: '/settings', label: '面板' },
    { path: '/settings/dns', label: 'DNS' },
    { path: '/settings/advanced', label: '高级' },
  ],
};

// ─── Forwarding Mode Labels ───
export const FORWARDING_MODE_LABELS: Record<ForwardingMode, string> = {
  reverse_proxy: '反向代理',
  load_balance: '负载均衡',
  relay: '中继转发',
  maintenance: '维护模式',
  transparent_proxy: '透明代理',
};

// ─── Pipeline Phase Order ───
export const PIPELINE_PHASES = ['match', 'gateway', 'target', 'forwarding', 'fallback', 'health'] as const;

// ─── Legacy URL Redirect Map ───
export const LEGACY_REDIRECTS: Record<string, string> = {
  '/routes': '/exposure',
  '/services': '/exposure',
  '/endpoints': '/exposure',
  '/gateways': '/fabric',
  '/gateway-links': '/fabric',
  '/topology': '/fabric',
  '/topology/path': '/fabric',
  '/routing': '/fabric',
  '/policies': '/fabric',
  '/providers': '/fabric',
  '/middleware': '/fabric',
  '/listeners': '/fabric',
  '/nodes': '/runtime',
  '/sync': '/runtime/sync',
  '/join-tokens': '/access/tokens',
  '/apply': '/release',
  '/config': '/release',
  '/import': '/exposure',
  '/quick-create': '/exposure/new',
  '/trace': '/observe',
  '/health': '/observe/health',
  '/safety': '/observe/safety',
  '/doctor': '/observe/doctor',
  '/smoke': '/observe/doctor',
  '/acceptance': '/observe/acceptance',
  '/logs': '/observe/logs',
  '/audit': '/observe/audit',
  '/relay': '/observe',
  '/scopes': '/access/admin',
  '/credentials': '/access/credentials',
  '/exposures': '/exposure',
  '/settings': '/settings',
  '/transparent': '/settings/proxy',
  '/security': '/settings/advanced',
  '/maintenance': '/settings/advanced',
  '/actions': '/settings/advanced',
  '/fabric/auth': '/auth',
  '/fabric/callgraph': '/auth/callgraph',
};

// ─── Workspace Icons (inline SVG paths) ───
export const WORKSPACE_ICONS: Record<string, string> = {
  dashboard: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z',
  exposure: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z',
  fabric: 'M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z',
  runtime: 'M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h6v2H8v2h8v-2h-2v-2h6c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H4V4h16v12z',
  release: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z',
  observe: 'M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z',
  shield: 'M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 10.99h7c-.53 4.12-3.28 7.79-7 8.94V12H5V6.3l7-3.11v8.8z',
  access: 'M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 10.99h7c-.53 4.12-3.28 7.79-7 8.94V12H5V6.3l7-3.11v8.8z',
  settings: 'M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94L14.4 2.81c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41L9.25 5.35c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.62-.07.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z',
};
