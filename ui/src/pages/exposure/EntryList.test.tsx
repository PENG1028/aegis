import { http, HttpResponse } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EntryList from './EntryList';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

const route = {
  id: 'rt_api', domain: 'api.example.com', service_id: 'svc_api', composition: 'https_route',
  status: 'active', owner_type: 'admin', tls_enabled: true, tls_binding_mode: 'certificate',
  cert_id: 'cert_shared', created_at: '2026-01-01T00:00:00Z',
};

function registerListHandlers(deletePreview: Record<string, unknown>, onDelete?: (url: URL) => void) {
  server.use(
    http.get('*/api/system/runtime-mode', () => HttpResponse.json({
      current: { id: 'legacy', label: 'Legacy', providers: [], compositions: [{ name: 'HTTPS Route', status: 'available' }] },
      available_modes: [],
    })),
    http.get('*/api/admin/v1/routes', () => HttpResponse.json({ routes: [route], count: 1 })),
    http.get('*/api/exposures', () => HttpResponse.json({ exposures: [], count: 0 })),
    http.get('*/api/admin/v1/services', () => HttpResponse.json({ services: [{ id: 'svc_api', kind: 'http' }] })),
    http.get('*/api/admin/v1/certificates', () => HttpResponse.json({
      certificates: [{ id: 'cert_shared', source: 'manual_upload' }], assets: [], automatic_tls: [],
    })),
    http.get('*/api/admin/v1/routes/rt_api/delete-preview', () => HttpResponse.json(deletePreview)),
    http.delete('*/api/admin/v1/routes/rt_api', ({ request }) => {
      onDelete?.(new URL(request.url));
      return HttpResponse.json({ status: 'deleted', certificate_deleted: false });
    }),
  );
}

describe('EntryList route and certificate lifecycle', () => {
  it('retains the certificate by default and sends cleanup only after explicit selection', async () => {
    let deletedURL: URL | undefined;
    registerListHandlers({
      action: 'delete_route', allowed: true, route_id: 'rt_api', domain: route.domain,
      tls_binding_mode: 'certificate', certificate_id: 'cert_shared',
      delete_unused_certificate_allowed: true,
      effects: ['delete route', 'retain certificate by default'],
    }, url => { deletedURL = url; });
    const user = userEvent.setup();
    renderPage(<EntryList />, '/exposure');

    await user.click(await screen.findByRole('button', { name: '删除' }));
    const cleanup = await screen.findByRole('checkbox');
    expect(cleanup).not.toBeChecked();
    expect(screen.getByText(/证书资产会保留/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '删除域名' }));
    await waitFor(() => expect(deletedURL).toBeDefined());
    expect(deletedURL?.searchParams.get('delete_unused_certificate')).toBe('false');
  });

  it('does not offer certificate cleanup when the asset is shared', async () => {
    registerListHandlers({
      action: 'delete_route', allowed: true, route_id: 'rt_api', domain: route.domain,
      tls_binding_mode: 'certificate', certificate_id: 'cert_shared',
      delete_unused_certificate_allowed: false,
      effects: ['delete route', 'retain certificate by default'],
    });
    const user = userEvent.setup();
    renderPage(<EntryList />, '/exposure');

    await user.click(await screen.findByRole('button', { name: '删除' }));
    await screen.findByText(/证书资产会保留/);
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByText('同时删除不再使用的证书资产')).not.toBeInTheDocument();
  });
});
