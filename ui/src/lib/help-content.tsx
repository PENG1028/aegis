/**
 * Help Content — all page/documentation help text.
 *
 * Each entry is a { title, content } pair rendered in HelpModal.
 * Content uses JSX for rich formatting (code, bold, lists, links).
 */

import type { ReactNode } from 'react';

interface HelpEntry {
  title: string;
  content: ReactNode;
}

export const HELP: Record<string, HelpEntry> = {

  // ─── Dashboard ───
  dashboard: {
    title: '总览',
    content: (
      <div className="space-y-3">
        <p><strong>总览</strong> 页面展示 Aegis 系统的整体运行状态。</p>
        <div>
          <p className="font-medium mb-1">统计卡片：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>节点</strong> — 注册到控制面的服务器数量</li>
            <li><strong>路由</strong> — 已配置的域名路由数量</li>
            <li><strong>Gateway Links</strong> — 节点间安全通信链路数量</li>
          </ul>
        </div>
        <div>
          <p className="font-medium mb-1">常用操作：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>点击 <strong>Apply 管理</strong> 查看待应用的配置变更</li>
            <li>查看 <strong>节点状态</strong> 了解各节点是否在线</li>
            <li>查看 <strong>最近失败</strong> 快速定位问题</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Nodes ───
  nodes: {
    title: '节点管理',
    content: (
      <div className="space-y-3">
        <p><strong>节点</strong> 是接入 Aegis 控制面的每一台服务器。</p>
        <div>
          <p className="font-medium mb-1">节点角色：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>控制面节点</strong> — 运行 aegis serve，管理配置和 Apply</li>
            <li><strong>网关节点</strong> — 运行 Caddy/HAProxy，处理流量转发</li>
            <li><strong>Relay 目标节点</strong> — 接受来自其他节点的中继请求</li>
          </ul>
        </div>
        <div>
          <p className="font-medium mb-1">状态说明：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>online</strong> — 节点心跳正常</li>
            <li><strong>offline</strong> — 超过心跳超时未上报</li>
            <li><strong>degraded</strong> — 部分组件异常</li>
          </ul>
        </div>
        <p>添加新节点需先在 <strong>Join Tokens</strong> 页面创建加入令牌。</p>
      </div>
    ),
  },

  // ─── Join Tokens ───
  'join-tokens': {
    title: 'Join Tokens',
    content: (
      <div className="space-y-3">
        <p><strong>Join Token</strong> 是让新节点注册到控制面的一次性凭证。</p>
        <div>
          <p className="font-medium mb-1">使用流程：</p>
          <ol className="list-decimal list-inside space-y-1 text-a-fg2">
            <li>在此页面创建 Join Token，指定名称和过期时间</li>
            <li>在目标服务器上运行 <code className="text-a-accent">aegis join &lt;token&gt;</code></li>
            <li>节点自动注册到控制面，开始心跳上报</li>
          </ol>
        </div>
        <p className="text-a-warn">⚠ Join Token 只展示一次，关闭后无法再次查看。</p>
      </div>
    ),
  },

  // ─── Gateways ───
  gateways: {
    title: '网关管理',
    content: (
      <div className="space-y-3">
        <p><strong>Gateway</strong> 是 Aegis 对 Caddy/HAProxy 的抽象管理接口。</p>
        <div>
          <p className="font-medium mb-1">三种类型：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>local</strong> — 本机网关(:18080)，用于调试和开发者入口</li>
            <li><strong>private</strong> — 内网网关，仅内网可达</li>
            <li><strong>public</strong> — 公网网关，接收外部流量</li>
          </ul>
        </div>
        <p>通常不需要手动管理 Gateway，创建 Route 并 Apply 后系统会自动分配。</p>
      </div>
    ),
  },

  // ─── Services ───
  services: {
    title: '服务管理',
    content: (
      <div className="space-y-3">
        <p><strong>Service</strong> 代表一个后端应用，是 Aegis 路由转发的目标。</p>
        <div>
          <p className="font-medium mb-1">基本概念：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>每个 Service 有一个或多个 <strong>Endpoint</strong>（实际监听地址）</li>
            <li>Route 将域名指向 Service，Service 再将请求转发给 Endpoint</li>
            <li>Service 可以分布在不同的节点上</li>
          </ul>
        </div>
        <div>
          <p className="font-medium mb-1">使用流程：</p>
          <ol className="list-decimal list-inside space-y-1 text-a-fg2">
            <li>创建 Service（如 <code>my-blog</code>）</li>
            <li>为 Service 添加 Endpoint（如 <code>node-b:2368</code>）</li>
            <li>创建 Route 将域名指向此 Service</li>
          </ol>
        </div>
      </div>
    ),
  },

  // ─── Routes ───
  routes: {
    title: '路由管理',
    content: (
      <div className="space-y-3">
        <p><strong>Route</strong> 将域名映射到后端 Service，是流量入口的核心配置。</p>
        <div>
          <p className="font-medium mb-1">关键字段：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>Domain</strong> — 域名，如 <code>blog.example.com</code></li>
            <li><strong>Service</strong> — 目标服务</li>
            <li><strong>TLS Mode</strong> — HTTPS 处理方式（终止/直通/仅HTTP）</li>
            <li><strong>Public Allowed</strong> — 是否允许通过公网网关访问</li>
          </ul>
        </div>
        <p className="text-a-warn">⚠ 修改 Route 后需要执行 <strong>Apply</strong> 才能生效。</p>
      </div>
    ),
  },

  // ─── Endpoints ───
  endpoints: {
    title: '端点管理',
    content: (
      <div className="space-y-3">
        <p><strong>Endpoint</strong> 是 Service 的实际监听地址，格式为 <code>IP:端口</code>。</p>
        <div>
          <p className="font-medium mb-1">地址类型：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>local</strong> — 本机地址（127.0.0.1 或本机内网 IP）</li>
            <li><strong>remote</strong> — 跨节点地址（通过 Relay 转发）</li>
          </ul>
        </div>
        <p>一个 Service 可以有多个 Endpoint 分布在不同的节点上，实现负载均衡。</p>
      </div>
    ),
  },

  // ─── Topology ───
  topology: {
    title: '网络拓扑',
    content: (
      <div className="space-y-3">
        <p><strong>Topology</strong> 展示各节点之间的网络连通状态。</p>
        <div>
          <p className="font-medium mb-1">状态类型：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>verified</strong> — 双向连通正常</li>
            <li><strong>missing_link</strong> — 缺少 GatewayLink</li>
            <li><strong>unreachable</strong> — 网络不可达</li>
            <li><strong>degraded</strong> — 部分可达</li>
          </ul>
        </div>
        <p><strong>路径查询</strong> 可以查看从指定节点到另一个节点的完整转发路径。</p>
      </div>
    ),
  },

  // ─── Trace ───
  trace: {
    title: 'Trace',
    content: (
      <div className="space-y-3">
        <p><strong>Trace</strong> 追踪一个请求从进入到到达目标的完整路径。</p>
        <div>
          <p className="font-medium mb-1">两种模式：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>Ingress Trace</strong> — 输入域名/SNI/Route ID，追踪匹配到的路由和转发路径</li>
            <li><strong>Egress Trace</strong> — 检查目标域名的外网出口路径，判断是否存在安全风险</li>
          </ul>
        </div>
        <p>Trace 是只读分析工具，不修改任何配置。</p>
      </div>
    ),
  },

  // ─── Safety ───
  safety: {
    title: 'Route Safety',
    content: (
      <div className="space-y-3">
        <p><strong>Safety</strong> 检查所有路由的安全配置，发现潜在的流量泄漏风险。</p>
        <div>
          <p className="font-medium mb-1">检查项：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>目标 IP 分类（公网/内网/本机地址）</li>
            <li>GatewayLink 是否必需的但没有配置</li>
            <li>是否指向本机 Gateway Listener（自环风险）</li>
            <li>跨节点流量是否正确使用 Relay</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Relay ───
  relay: {
    title: 'Relay',
    content: (
      <div className="space-y-3">
        <p><strong>Relay</strong> 是 Aegis 的跨节点中继转发机制。</p>
        <div>
          <p className="font-medium mb-1">为什么需要 Relay：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>节点 A 上的服务想访问节点 B 上的服务</li>
            <li>通过 GatewayLink 加密认证后，Relay 将请求转发到目标节点的 Endpoint</li>
            <li>目标节点的真实端口会被隐藏（Direct Target Suppressed）</li>
          </ul>
        </div>
        <p><strong>Resolve</strong> 可以测试某个域名经由 Relay 转发的完整路径。</p>
      </div>
    ),
  },

  // ─── Gateway Links ───
  'gateway-links': {
    title: 'Gateway Links',
    content: (
      <div className="space-y-3">
        <p><strong>Gateway Link</strong> 是两个节点之间的加密认证通道。</p>
        <div>
          <p className="font-medium mb-1">作用：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>节点间通信的认证和加密</li>
            <li>使用 HMAC-SHA256 签名 + 时间戳防重放</li>
            <li>Secret 在数据库中 AES-256-GCM 加密存储</li>
          </ul>
        </div>
        <p className="text-a-warn">⚠ Secret 只展示一次，关闭后无法再次查看。创建后建议立即复制保存。</p>
      </div>
    ),
  },

  // ─── Gateway Policies ───
  policies: {
    title: 'Gateway Policies',
    content: (
      <div className="space-y-3">
        <p><strong>Gateway Policy</strong> 控制流量经过哪些网关、是否需要 Relay。</p>
        <div>
          <p className="font-medium mb-1">四种模式：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>auto</strong> — 自动选择最优路径</li>
            <li><strong>fixed</strong> — 固定使用指定网关</li>
            <li><strong>multi</strong> — 多个候选网关，主备切换</li>
            <li><strong>disabled</strong> — 不使用网关，直连目标</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Routing Table ───
  routing: {
    title: 'Routing Table',
    content: (
      <div className="space-y-3">
        <p><strong>Routing Table</strong> 显示每个节点上的路由条目和候选转发路径。</p>
        <div>
          <p className="font-medium mb-1">三个 Tab：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>Table</strong> — 当前路由条目列表，包含所有候选网关</li>
            <li><strong>Preview</strong> — 预览指定域名的路由选择结果</li>
            <li><strong>Validate</strong> — 校验路由配置的合法性和完整性</li>
          </ul>
        </div>
        <p>路由表由系统自动生成，通常不需要手动修改。</p>
      </div>
    ),
  },

  // ─── Sync Status ───
  sync: {
    title: 'Sync Status',
    content: (
      <div className="space-y-3">
        <p><strong>Sync Status</strong> 展示控制面（Desired State）和节点（Actual State）之间的状态同步情况。</p>
        <div>
          <p className="font-medium mb-1">状态说明：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>in_sync</strong> — Desired = Actual，一致</li>
            <li><strong>outdated</strong> — 节点落后于控制面，需要同步</li>
            <li><strong>failed</strong> — 同步失败，需排查节点网络或配置</li>
            <li><strong>no_desired_state</strong> — 该节点尚未分配期望状态</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Local Gateway ───
  'local-gateway': {
    title: 'Local Gateway',
    content: (
      <div className="space-y-3">
        <p><strong>Local Gateway</strong> 是运行在每个节点上的本地 HTTP 网关（:18080）。</p>
        <div>
          <p className="font-medium mb-1">用途：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>本地开发者入口，调试和测试用</li>
            <li>承载本地路由表，直接转发到本机 Endpoint</li>
            <li>不需要经过公网网关或 Relay</li>
          </ul>
        </div>
        <p className="text-a-warn">⚠ 开发者入口模式（A/B/C）仅用于开发和调试，生产环境应禁用。</p>
      </div>
    ),
  },

  // ─── Acceptance ───
  acceptance: {
    title: '验证状态',
    content: (
      <div className="space-y-3">
        <p><strong>验证状态</strong> 汇总了 Aegis 系统的各项验收测试结果。</p>
        <div>
          <p className="font-medium mb-1">标签状态：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>pass</strong> — 已验证通过</li>
            <li><strong>pending</strong> — 待验证（功能已实现但未在真实环境验收）</li>
            <li><strong>deferred</strong> — 已延期（计划在后续版本验证）</li>
          </ul>
        </div>
        <p><strong>Negative Smoke</strong> 是安全负面测试，验证非法请求是否被正确拦截。</p>
      </div>
    ),
  },

  // ─── Apply/Config ───
  apply: {
    title: 'Apply',
    content: (
      <div className="space-y-3">
        <p><strong>Apply</strong> 将控制面的配置推送到 Caddy/HAProxy 使其生效。</p>
        <div>
          <p className="font-medium mb-1">流程：</p>
          <ol className="list-decimal list-inside space-y-1 text-a-fg2">
            <li>创建/修改 Route、Service 等资源（此时只写入数据库）</li>
            <li>在 Apply 页面查看待应用的变更</li>
            <li>执行 Apply → 渲染配置 → 校验 → 原子替换 → 重载 Provider</li>
          </ol>
        </div>
        <p className="text-a-warn">⚠ 修改配置后 <strong>必须 Apply</strong> 才能生效。Apply 失败时系统会自动回滚。</p>
      </div>
    ),
  },

  // ─── Config ───
  config: {
    title: 'Config',
    content: (
      <div className="space-y-3">
        <p><strong>Config</strong> 展示 Caddy/HAProxy 的渲染结果，是只读的证据页面。</p>
        <div>
          <p className="font-medium mb-1">用途：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>查看当前生效的配置文件内容</li>
            <li>对比 Apply 前后的配置差异</li>
            <li>排查配置问题（如语法错误、缺失路由等）</li>
          </ul>
        </div>
        <p>配置由系统根据 Route 和 Service 自动生成，无需手动编辑。</p>
      </div>
    ),
  },

  // ─── Providers ───
  providers: {
    title: 'Providers',
    content: (
      <div className="space-y-3">
        <p><strong>Provider</strong> 是 Aegis 支持的网关软件适配层。</p>
        <div>
          <p className="font-medium mb-1">已支持：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>Caddy HTTP</strong> — 反向代理、TLS 终止</li>
            <li><strong>HAProxy TCP</strong> — SNI 多路复用、TCP 转发</li>
          </ul>
        </div>
        <p><strong>诊断</strong> 功能检查 Provider 是否安装、配置文件是否存在、服务是否运行。</p>
      </div>
    ),
  },

  // ─── Scopes ───
  scopes: {
    title: 'Scopes',
    content: (
      <div className="space-y-3">
        <p><strong>Scope</strong> 是资源的逻辑隔离单元，相当于一个「租户空间」。</p>
        <div>
          <p className="font-medium mb-1">设计目的：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>不同 Scope 之间的 Service、Route、Edge Rule 完全隔离</li>
            <li>API Key 绑定到 Scope 后，只能操作该 Scope 内的资源</li>
            <li>每个 Scope 有独立的配额限制（路由数、服务数等）</li>
          </ul>
        </div>
        <p>如果是单管理员使用，只需要一个 Scope 即可。</p>
      </div>
    ),
  },

  // ─── API Keys ───


  // ─── Logs ───
  logs: {
    title: '日志',
    content: (
      <div className="space-y-3">
        <p>系统提供两种日志，分别关注不同的维度。</p>
        <div>
          <p className="font-medium mb-1">Op Logs（操作日志）</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>记录你对资源的所有操作：创建路由、修改服务、执行 Apply 等</li>
            <li>用于日常运维排查和变更追踪</li>
          </ul>
        </div>
        <div>
          <p className="font-medium mb-1">Audit Logs（审计日志）</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>记录所有认证和授权事件：登录成功/失败、令牌无效、越权访问</li>
            <li>包含来源 IP 和 User-Agent，用于安全审计</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── QuickCreate ───
  'quick-create': {
    title: '快速创建映射',
    content: (
      <div className="space-y-3">
        <p><strong>快速创建</strong> 是创建域名映射的最简路径。</p>
        <div>
          <p className="font-medium mb-1">使用方式：</p>
          <ol className="list-decimal list-inside space-y-1 text-a-fg2">
            <li>输入域名（如 <code>blog.example.com</code>）</li>
            <li>输入目标主机和端口（如 <code>127.0.0.1:2368</code>）</li>
            <li>选择目标节点</li>
            <li>点击 <strong>创建映射</strong></li>
          </ol>
        </div>
        <p>系统会自动创建 Service → Endpoint → Route，之后需 Apply 生效。</p>
        <p className="text-a-muted text-[11px] mt-2">高级选项（TLS、Gateway Policy）默认折叠，有需要可以展开配置。</p>
      </div>
    ),
  },

  // ─── HealthCheck ───
  health: {
    title: '健康检查',
    content: (
      <div className="space-y-3">
        <p><strong>健康检查</strong> 发送请求到每个 Endpoint 检查服务是否正常运行。</p>
        <div>
          <p className="font-medium mb-1">状态含义：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li><strong>healthy</strong> — 服务正常响应</li>
            <li><strong>unhealthy</strong> — 服务未正常响应</li>
            <li><strong>unknown</strong> — 无法确认（可能未配置健康检查）</li>
          </ul>
        </div>
        <p>如果 Endpoint 状态异常，检查目标服务是否运行、端口是否正确、防火墙是否放行。</p>
      </div>
    ),
  },

  // ─── Import Config ───
  'import-config': {
    title: '导入配置',
    content: (
      <div className="space-y-3">
        <p><strong>导入配置</strong> 扫描服务器上已有的 Caddyfile，提取域名和后端地址映射。</p>
        <div>
          <p className="font-medium mb-1">使用流程：</p>
          <ol className="list-decimal list-inside space-y-1 text-a-fg2">
            <li>点击 <strong>扫描 Caddyfile</strong>，系统自动查找并解析 Caddyfile</li>
            <li>预览发现的路由，勾选需要导入的条目</li>
            <li>点击 <strong>导入</strong>，写入 Aegis 数据库（创建 Service + Endpoint + Route）</li>
            <li>去 Apply 页面执行 Apply 使之生效</li>
          </ol>
        </div>
        <p className="text-a-warn">⚠ 导入后不会自动覆盖已运行的 Caddy 配置，需要 Apply。</p>
      </div>
    ),
  },

  // ─── Settings ───
  settings: {
    title: '设置',
    content: (
      <div className="space-y-3">
        <p><strong>Settings</strong> 展示系统的当前配置。</p>
        <p>此页面为只读视图，显示：</p>
        <ul className="list-disc list-inside space-y-1 text-a-fg2">
          <li>Admin 会话配置</li>
          <li>Gateway 默认参数</li>
          <li>Relay 转发配置</li>
          <li>Safety 安全检查规则</li>
        </ul>
        <p>配置修改需要通过服务端配置文件进行。</p>
      </div>
    ),
  },

  // ─── Listeners ───
  listeners: {
    title: 'Listeners',
    content: (
      <div className="space-y-3">
        <p><strong>Listeners</strong> 展示系统当前监听的所有端口和协议。</p>
        <div>
          <p className="font-medium mb-1">用途：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>确认 Provider 是否正确绑定了端口</li>
            <li>排查端口冲突问题</li>
            <li>了解当前系统的网络暴露面</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Doctor ───
  doctor: {
    title: 'Doctor',
    content: (
      <div className="space-y-3">
        <p><strong>Doctor</strong> 是系统的部署检查工具，类似 <code>aegis doctor</code> 命令。</p>
        <p>检查内容包括：</p>
        <ul className="list-disc list-inside space-y-1 text-a-fg2">
          <li>Provider 是否安装且版本正确</li>
          <li>配置文件是否合法</li>
          <li>服务是否正常运行</li>
          <li>端口监听是否正常</li>
        </ul>
      </div>
    ),
  },

  // ─── Smoke ───
  smoke: {
    title: 'Smoke',
    content: (
      <div className="space-y-3">
        <p><strong>Smoke</strong> 展示冒烟测试的执行结果。</p>
        <p>冒烟测试是一组快速验证，确保系统核心功能正常：</p>
        <ul className="list-disc list-inside space-y-1 text-a-fg2">
          <li>Golden Path — 基础流量路径</li>
          <li>Provider Smoke — 网关软件状态</li>
          <li>Relay Resolve — 跨节点中继解析</li>
          <li>Negative Test — 安全拦截验证</li>
        </ul>
      </div>
    ),
  },

  // ─── Security ───
  security: {
    title: 'Security',
    content: (
      <div className="space-y-3">
        <p><strong>Security</strong> 页面展示密钥存储和令牌边界的安全状态。</p>
        <div>
          <p className="font-medium mb-1">安全措施：</p>
          <ul className="list-disc list-inside space-y-1 text-a-fg2">
            <li>GatewayLink Secret — AES-256-GCM 加密存储</li>
            <li>API Key — SHA-256 哈希，不落明文</li>
            <li>Admin Session — HttpOnly Cookie，24h 过期</li>
            <li>所有鉴权失败记录 Audit Log</li>
          </ul>
        </div>
      </div>
    ),
  },

  // ─── Maintenance ───
  maintenance: {
    title: 'Maintenance',
    content: (
      <div className="space-y-3">
        <p><strong>Maintenance</strong> 提供系统维护操作入口。</p>
        <ul className="list-disc list-inside space-y-1 text-a-fg2">
          <li><strong>SQLite Backup</strong> — 备份数据库</li>
          <li><strong>WAL Checkpoint</strong> — 压缩 WAL 日志</li>
          <li><strong>Diagnostics Export</strong> — 导出诊断信息</li>
          <li><strong>Config Snapshot</strong> — 创建配置快照</li>
        </ul>
        <p className="text-a-warn">⚠ 维护操作可能影响系统运行，谨慎执行。</p>
      </div>
    ),
  },

  // ─── Actions ───
  actions: {
    title: 'Actions',
    content: (
      <div className="space-y-3">
        <p><strong>Actions</strong> 提供受控的操作入口，避免直接编辑原始配置。</p>
        <ul className="list-disc list-inside space-y-1 text-a-fg2">
          <li><strong>Bind HTTP Domain</strong> — 绑定域名到 HTTP 目标</li>
          <li><strong>Bind TLS Backend</strong> — 绑定 SNI Host</li>
          <li><strong>Update Target</strong> — 更新目标地址</li>
          <li><strong>Relay Resolve Test</strong> — 测试 Relay 路径</li>
        </ul>
        <p>每个 Action 都会记录操作日志，方便追溯。</p>
      </div>
    ),
  },

};
