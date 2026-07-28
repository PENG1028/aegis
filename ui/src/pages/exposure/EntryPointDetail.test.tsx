import { http, HttpResponse } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EntryPointDetail from './EntryPointDetail';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

const route = {
  id: 'rt_api', domain: 'api.example.com', service_id: 'svc_api', composition: 'https_route',
  source_provider: 'executor-a', source_capabilities: '["route_host","auto_cert"]',
  status: 'active', owner_type: 'admin', tls_enabled: true,
  tls_binding_mode: 'provider_auto', tls_provider: 'executor-a', cert_id: '',
  path_prefix: '/', created_at: '2026-01-01T00:00:00Z',
};

function registerHandlers(availability: 'ready' | 'degraded', counts?: Record<string, number>) {
  const count = (key: string) => { if (counts) counts[key] = (counts[key] || 0) + 1; };
  server.use(
    http.get('*/api/admin/v1/routes/rt_api', () => { count('route'); return HttpResponse.json(route); }),
    http.get('*/api/admin/v1/services/svc_api', () => HttpResponse.json({ id: 'svc_api', name: 'API', kind: 'http' })),
    http.get('*/api/admin/v1/services/svc_api/endpoints', () => HttpResponse.json({ endpoints: [{ id: 'ep', address: '127.0.0.1:3000', type: 'http', enabled: true }] })),
    http.get('*/api/admin/v1/certificates', () => { count('certificates'); return HttpResponse.json({
      certificates: [{
        id: 'cert_api', domains: '["api.example.com"]', issuer: 'Test CA',
        not_before: '2026-01-01T00:00:00Z', not_after: '2030-01-01T00:00:00Z',
        source: 'manual_upload', record_type: 'asset', auto_renew: false,
      }],
    }); }),
    http.get('*/api/system/runtime-mode', () => { count('runtime'); return HttpResponse.json({
      current: { label: 'Active', providers: [{ provider_id: 'executor-a' }], compositions: [{ name: 'HTTPS Route', status: 'available' }] },
      available_modes: [],
    }); }),
    http.get('*/api/admin/v1/providers', () => { count('providers'); return HttpResponse.json({ providers: [{
      id: 'executor-a', installed: true, running: availability === 'ready', capabilities: ['auto_cert', 'load_cert'],
      capability_statuses: ['auto_cert', 'load_cert'].map(key => ({ key, availability, semantics: { state_class: key === 'auto_cert' ? 'provider_managed' : 'portable_asset', affinity: key === 'auto_cert' ? 'executor_sticky' : 'none', migration: key === 'auto_cert' ? 'recreate' : 'reload_asset' } })),
    }] }); }),
    http.get('*/api/admin/v1/routes/rt_api/capability-status', () => { count('capability'); return HttpResponse.json({
      route_id: 'rt_api', provider: 'executor-a', operations: { modify: { available: true }, delete: { available: true } },
    }); }),
    http.put('*/api/admin/v1/routes/rt_api/tls-binding', async ({ request }) => {
      count('mutation');
      return HttpResponse.json({ status: 'updated', request: await request.json() });
    }),
  );
}

describe('EntryPointDetail capability projection', () => {
  it.each([
    ['ready', '路由与 TLS 能力均已就绪'],
    ['degraded', 'TLS 执行能力不可用，需要恢复原执行器或显式迁移'],
  ] as const)('renders automatic TLS executor state: %s', async (availability, message) => {
    registerHandlers(availability);
    renderPage(<EntryPointDetail />, '/exposure/entry/rt_api', '/exposure/entry/:entryId');

    expect(await screen.findByRole('heading', { name: 'api.example.com' })).toBeInTheDocument();
    expect(screen.getByText('自动 TLS · executor-a')).toBeInTheDocument();
    expect(screen.getByText(message)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /证书管理/ })).toHaveAttribute('href', '/access/certificates');
    expect(screen.getByRole('link', { name: /模式管理/ })).toHaveAttribute('href', '/fabric/mode');
  });

  it('sends the selected asset binding and refreshes every dependent state query', async () => {
    const counts: Record<string, number> = {};
    registerHandlers('ready', counts);
    const user = userEvent.setup();
    renderPage(<EntryPointDetail />, '/exposure/entry/rt_api', '/exposure/entry/:entryId');
    await screen.findByText('自动 TLS · executor-a');

    await user.click(screen.getByRole('button', { name: '更改绑定' }));
    await user.selectOptions(screen.getByLabelText('管理方式'), 'cert_api');
    await user.click(screen.getByRole('button', { name: '应用绑定' }));

    await waitFor(() => expect(counts.mutation).toBe(1));
    await waitFor(() => {
      expect(counts.route).toBeGreaterThan(1);
      expect(counts.certificates).toBeGreaterThan(1);
      expect(counts.capability).toBeGreaterThan(1);
      expect(counts.providers).toBeGreaterThan(1);
      expect(counts.runtime).toBeGreaterThan(1);
    });
  });
});
