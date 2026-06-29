/**
 * Sidebar — consolidated navigation.
 *
 * 32 items → ~16 items. Grouped by user task, not by backend module.
 * Related pages merged into single entries (use tabs inside the page).
 */

import { useLocation, useNavigate } from 'react-router-dom';

interface NavItem {
  href: string;
  label: string;
  icon: string;
  disabled?: boolean;
}

interface NavSection {
  label: string | null;
  items: NavItem[];
}

const SECTIONS: NavSection[] = [
  {
    label: null,
    items: [
      { href: '/', label: '总览',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1" y="1" width="6" height="6" rx="1"/><rect x="9" y="1" width="6" height="6" rx="1"/><rect x="1" y="9" width="6" height="6" rx="1"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg>' },
    ],
  },
  {
    label: '快速操作',
    items: [
      { href: '/quick-create', label: '创建映射',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12"/></svg>' },
      { href: '/routes', label: '路由列表',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="6"/><path d="M2 8h12M8 2c1.5 2 2.5 4 2.5 6S9.5 14 8 14"/></svg>' },
      { href: '/health', label: '健康检查',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 14A6 6 0 108 2a6 6 0 000 12z"/><path d="M5.5 8l2 2 3-4"/></svg>' },
    ],
  },
  {
    label: '资源',
    items: [
      { href: '/services', label: '服务',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1" y="3" width="14" height="10" rx="2"/><circle cx="12" cy="8" r="1" fill="currentColor"/></svg>' },
      { href: '/endpoints', label: '端点',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="1.5"/><path d="M2 8h4M10 8h4M8 2v4M8 10v4"/></svg>' },
    ],
  },
  {
    label: '网络',
    items: [
      { href: '/topology', label: '拓扑',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="6" cy="6" r="2.5"/><circle cx="10" cy="10" r="2.5"/><path d="M8 8l2 2"/></svg>' },
      { href: '/trace', label: '跟踪诊断',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="6" cy="6" r="2.5"/><circle cx="10" cy="10" r="2.5"/><path d="M8 8l2 2"/></svg>' },
      { href: '/transparent', label: '透明代理',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h3v8H2z"/><path d="M7 3h7v4H7z"/><path d="M7 9h4v3H7z"/><path d="M13 9h1v3h-1z"/></svg>' },
    ],
  },
  {
    label: '网关',
    items: [
      { href: '/gateways', label: '网关列表',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 8h12M8 2v12"/><circle cx="5" cy="5" r="1.5"/><circle cx="11" cy="11" r="1.5"/></svg>' },
      { href: '/gateway-links', label: '网关链接',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 8h10M8 3v10"/><circle cx="5" cy="5" r="1.5"/><circle cx="11" cy="11" r="1.5"/></svg>' },
    ],
  },
  {
    label: '策略',
    items: [
      { href: '/policies', label: '策略规则',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 2v12l4-2 4 2V2l-4 2-4-2z"/></svg>' },
      { href: '/routing', label: '路由表',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 3h10v10H3z"/><path d="M6 6h4M6 9h3"/></svg>' },
    ],
  },
  {
    label: '运维',
    items: [
      { href: '/nodes', label: '节点',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1" y="3" width="5" height="4" rx="1"/><rect x="10" y="3" width="5" height="4" rx="1"/><rect x="1" y="9" width="5" height="4" rx="1"/><rect x="10" y="9" width="5" height="4" rx="1"/></svg>' },
      { href: '/join-tokens', label: '加入令牌',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="10" height="10" rx="2"/><circle cx="8" cy="8" r="2"/></svg>' },
      { href: '/sync', label: '同步状态',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12"/></svg>' },
      { href: '/apply', label: '推送配置',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h4v2H2zM10 4h4v2h-4zM2 10h3v2H2zM9 10h5v2H9z"/></svg>' },
      { href: '/import', label: '导入配置',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12"/><path d="M5 8l3 3 3-3"/></svg>' },
    ],
  },
  {
    label: '诊断',
    items: [
      { href: '/doctor', label: '诊断工具',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 14A6 6 0 108 2a6 6 0 000 12z"/><path d="M5.5 8l2 2 3-4"/></svg>' },
      { href: '/smoke', label: '冒烟测试',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 14c1-4 3-6 6-6s5 2 6 6"/></svg>' },
      { href: '/acceptance', label: '验证状态',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="6"/><path d="M5.5 8l2 2 3-4"/></svg>' },
    ],
  },
  {
    label: '安全',
    items: [
      { href: '/scopes', label: '作用域',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2l6 4-6 4-6-4 6-4z"/></svg>' },
      { href: '/api-keys', label: 'API 密钥',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2a4 4 0 014 4v1h1a1 1 0 011 1v4a1 1 0 01-1 1H3a1 1 0 01-1-1V8a1 1 0 011-1h1V6a4 4 0 014-4z"/></svg>' },
      { href: '/providers', label: '提供商',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="2.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.1 3.1l1.4 1.4M11.5 11.5l1.4 1.4M3.1 12.9l1.4-1.4M11.5 4.5l1.4-1.4"/></svg>' },
      { href: '/middleware', label: '中间件',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1" y="1" width="14" height="5" rx="1"/><rect x="1" y="10" width="14" height="5" rx="1"/><path d="M4 6v4M8 6v4M12 6v4"/></svg>' },
    ],
  },
  {
    label: '日志',
    items: [
      { href: '/logs', label: '操作日志',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 3h10v10H3z"/><path d="M6 6h4M6 9h3M6 12h2"/></svg>' },
      { href: '/audit', label: '审计日志',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 1v3M8 12v3M1 8h3M12 8h3"/><circle cx="8" cy="8" r="2"/></svg>' },
    ],
  },
  {
    label: null,
    items: [
      { href: '/settings', label: '设置',
        icon: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="2.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.1 3.1l1.4 1.4M11.5 11.5l1.4 1.4M3.1 12.9l1.4-1.4M11.5 4.5l1.4-1.4"/></svg>' },
    ],
  },
];

export function Sidebar({ pendingApply }: { pendingApply?: boolean }) {
  const location = useLocation();
  const navigate = useNavigate();

  const isActive = (href: string) => {
    const a = '/' + location.pathname.replace(/^\/+/, '').split('/')[0];
    const b = '/' + href.replace(/^\/+/, '').split('/')[0];
    return a === b;
  };

  return (
    <aside className="w-[220px] min-w-[220px] bg-a-surface border-r border-a-border flex flex-col h-screen overflow-hidden">
      {/* Logo */}
      <div className="px-4 h-12 flex items-center gap-2.5 border-b border-a-border shrink-0">
        <svg className="w-[22px] h-[22px] shrink-0 text-a-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <rect x="3" y="3" width="18" height="18" rx="3" />
          <path d="M8 12h8M12 8v8" />
        </svg>
        <span className="font-mono text-sm font-bold tracking-[0.02em]">Aegis</span>
        <small className="font-mono text-[10px] text-a-muted ml-auto">v1.8C</small>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto p-2 flex flex-col gap-0.5 scrollbar-thin">
        {SECTIONS.map((section, i) => (
          <div key={i}>
            {section.label && (
              <div className="mt-3 px-2 pb-1 first:mt-0">
                <div className="text-[10px] font-semibold uppercase tracking-[0.06em] text-a-muted">
                  {section.label}
                </div>
              </div>
            )}
            {section.items.map((item) => {
              const active = isActive(item.href);
              return (
                <button
                  key={item.label}
                  onClick={() => !item.disabled && navigate(item.href)}
                  disabled={item.disabled}
                  className={`flex items-center gap-2.5 px-2.5 py-2 rounded-a-sm text-xs cursor-pointer transition-colors w-full text-left font-body border-none disabled:cursor-not-allowed ${
                    item.disabled
                      ? 'opacity-35 text-a-muted pointer-events-none'
                      : active
                      ? 'bg-a-accent/20 text-a-accent'
                      : 'text-a-muted hover:bg-a-border-soft hover:text-a-fg'
                  }`}
                >
                  <span className="w-4 h-4 shrink-0 opacity-70 flex items-center justify-center" dangerouslySetInnerHTML={{ __html: item.icon }} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        ))}
      </nav>

      {/* Status indicator */}
      <div className="p-3 border-t border-a-border shrink-0 flex items-center gap-2 text-[11px] text-a-muted">
        <span className="w-1.5 h-1.5 rounded-full bg-[#4cd964] animate-pulse"></span>
        {pendingApply ? '待应用' : 'Aegis 运行中'}
      </div>
    </aside>
  );
}
