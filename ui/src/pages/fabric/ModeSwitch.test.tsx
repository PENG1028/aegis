import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ModeSwitch from './ModeSwitch';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';
import type { ModeSwitchPreviewResponse } from '@/lib/real-api-client';

const preview: ModeSwitchPreviewResponse = {
  preview: {
    current_mode: 'legacy', target_mode: 'edge_mux', total_routes: 3,
    route_breakdown: [{ key: 'https_route', name: 'HTTPS Route', route_count: 3, current_mode_ok: true, target_mode_ok: true }],
    affected_routes: { kept: 3, unsupported: 0 }, provider_changes: [], risks: [],
  },
  rpcb_conflicts: [
    { route_id: 'rt_same', domain: 'same.example.com', capabilities: ['auto_cert'], compatible: true, current_executor: 'executor-a', target_executor: 'executor-a', state_class: 'provider_managed', migration: 're_render' },
    { route_id: 'rt_move', domain: 'move.example.com', capabilities: ['auto_cert'], compatible: true, current_executor: 'executor-a', target_executor: 'executor-b', state_class: 'provider_managed', migration: 'recreate' },
    { route_id: 'rt_asset', domain: 'asset.example.com', capabilities: ['load_cert'], compatible: true, current_executor: 'executor-a', target_executor: 'executor-b', state_class: 'portable_asset', migration: 'reload_asset' },
  ],
};

function registerHandlers(body = preview, switchStatus = 200) {
  server.use(
    http.get('*/api/system/runtime-mode', () => HttpResponse.json({
      current: { id: 'legacy', label: 'Legacy', compositions: [{ name: 'HTTPS Route', status: 'available' }] },
      available_modes: [{ id: 'legacy', label: 'Legacy', implemented: true }, { id: 'edge_mux', label: 'Edge Mux', description: 'Target', implemented: true }],
    })),
    http.get('*/api/admin/v1/providers', () => HttpResponse.json({ providers: [] })),
    http.post('*/api/admin/v1/mode/preview', () => HttpResponse.json(body)),
    http.post('*/api/admin/v1/mode/switch', () => switchStatus === 200
      ? HttpResponse.json({ status: 'success', message: 'done' })
      : HttpResponse.json({ status: 'failed', error: 'switch failed', rollback_status: 'complete' }, { status: switchStatus })),
  );
}

describe('ModeSwitch lifecycle preview', () => {
  it('renders migration strategy from the backend contract', async () => {
    registerHandlers();
    const user = userEvent.setup();
    renderPage(<ModeSwitch />, '/fabric/mode');
    await user.click(await screen.findByRole('button', { name: '切换到 Edge Mux' }));

    expect(await screen.findByText(/自动 TLS · 重新生成配置/)).toBeInTheDocument();
    expect(screen.getByText(/自动 TLS · 重新创建自动 TLS 状态/)).toBeInTheDocument();
    expect(screen.getByText(/证书资产 · 重新加载证书资产/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '确认切换' })).toBeEnabled();
  });

  it('blocks confirmation when the backend marks any route incompatible', async () => {
    registerHandlers({
      ...preview,
      rpcb_conflicts: [{ ...preview.rpcb_conflicts[0], compatible: false, migration: 'unsupported' as const, reason: 'missing capability' }],
    });
    const user = userEvent.setup();
    renderPage(<ModeSwitch />, '/fabric/mode');
    await user.click(await screen.findByRole('button', { name: '切换到 Edge Mux' }));

    expect(await screen.findByText('1 条入口无法迁移')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '确认切换' })).toBeDisabled();
  });

  it('shows automatic rollback status without suggesting a manual rollback', async () => {
    registerHandlers(preview, 500);
    const user = userEvent.setup();
    renderPage(<ModeSwitch />, '/fabric/mode');
    await user.click(await screen.findByRole('button', { name: '切换到 Edge Mux' }));
    await user.click(await screen.findByRole('button', { name: '确认切换' }));

    expect((await screen.findAllByText(/回滚状态：已自动恢复/)).length).toBeGreaterThan(0);
    expect(screen.queryByText(/POST \/api\/rollback/)).not.toBeInTheDocument();
  });
});
