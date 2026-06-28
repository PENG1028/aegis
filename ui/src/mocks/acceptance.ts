import type { AcceptanceStatus } from '@/types';

export const mockAcceptance: AcceptanceStatus = {
  labels: [
    { key: 'simulated_two_node_verified', label: '模拟双节点验证', status: 'pass', evidence: 'v1.8C-6B: 12 PASS / 1 DEFERRED' },
    { key: 'real_two_node_verified', label: '双节点中继验证', status: 'pass', evidence: 'Server A → Server B → target → HTTP 200' },
    { key: 'real_two_node_local_gateway_verified', label: '本地网关全路径', status: 'pass', evidence: 'curl → local-gw → relay → target HTTP 200' },
    { key: 'dev_entry_verified', label: '开发者入口验证', status: 'pass', evidence: '45 local gateway tests PASS' },
    { key: 'real_secret_runtime_code_verified', label: '密钥运行时代码验证', status: 'pass', evidence: '6 项集成测试 + 真实解密链' },
    { key: 'real_secret_runtime_deploy_pending', label: '密钥运行时部署验证', status: 'pending', evidence: '需通过新 API 创建加密 GatewayLink' },
    { key: 'real_three_node_pending', label: '三节点中继验证', status: 'pending', evidence: '未尝试' },
    { key: 'https_deferred', label: 'HTTPS 全透明', status: 'deferred', evidence: 'v2 规划' },
    { key: 'raw_tcp_deferred', label: 'Raw TCP 隧道', status: 'deferred', evidence: 'v2 规划' },
  ],
  summary: {
    total_labels: 9,
    pass_count: 5,
    pending_count: 2,
    deferred_count: 2,
  },
  last_acceptance: {
    command: 'curl -H "Host: api-b.example.com" http://127.0.0.1:18080/health',
    http_status: 200,
    response_summary: '{"service":"node-b-target","path":"/health","method":"POST","relay-target":"v18c8-test"}',
    selected_candidate: 'private_gateway (via Server B relay handler)',
    gateway_link_id: 'gl-a-b',
    token_leak_scan: 'clean',
    negative_smoke_result: 'pass',
    docs_link: 'docs/v1.8/real-two-node-vps-acceptance-result.md',
    executed_at: '2026-06-27T18:30:00Z',
  },
  negative_smoke: [
    { id: 'N1', desc: 'Wrong GatewayLink token', expected: '403/502', actual: '403 INVALID_GATEWAY_TOKEN', status: 'pass' },
    { id: 'N2', desc: 'Missing GatewayLink token', expected: '400', actual: '400 MISSING_GATEWAY_TOKEN', status: 'pass' },
    { id: 'N3', desc: 'Hop count exceeded (99)', expected: '508', actual: '508 MAX_HOPS_EXCEEDED', status: 'pass' },
    { id: 'N4', desc: 'Target header injection', expected: '400', actual: '400 TARGET_HEADER_REJECTED', status: 'pass' },
    { id: 'N5', desc: 'Target down (service stopped)', expected: '502', actual: '需端口冲突测试', status: 'partial' },
    { id: 'N6', desc: 'Direct remote fallback', expected: 'Blocked', actual: '未实现（仅 relay）', status: 'pass' },
    { id: 'N7', desc: 'Unmanaged domain rejection', expected: '421', actual: 'Handler 拒绝未知域名', status: 'pass' },
    { id: 'N8', desc: 'Raw token leak scan', expected: 'Clean', actual: '响应/错误消息中无 token', status: 'pass' },
  ],
};
