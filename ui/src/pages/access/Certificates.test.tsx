import { http, HttpResponse } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Certificates from './Certificates';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

const future = '2030-01-01T00:00:00Z';
const past = '2025-01-01T00:00:00Z';

const asset = {
  id: 'cert_wild', domains: '["*.example.com"]', issuer: 'Test CA',
  not_before: past, not_after: future, source: 'manual_upload', note: '',
  managed: true, managed_by: 'user', record_type: 'asset', ref_count: 1,
  auto_renew: false, created_at: past,
};

const observation = {
  id: 'managed_auto', domains: '["auto.example.com"]', issuer: 'Automatic CA',
  not_before: past, not_after: future, source: 'gateway_auto', note: '',
  managed: true, managed_by: 'executor-a', record_type: 'provider_observation',
  ref_count: 1, auto_renew: true, created_at: past,
};

function baseHandlers(extra: Parameters<typeof server.use>) {
  server.use(
    http.get('*/api/admin/v1/acme/status', () => HttpResponse.json({ available: true })),
    http.get('*/api/admin/v1/certificates', () => HttpResponse.json({
      certificates: [asset], assets: [asset], automatic_tls: [observation],
      count: 1, asset_count: 1, automatic_tls_count: 1,
      observation_warning: 'executor-b: storage unavailable',
    })),
    ...extra,
  );
}

describe('Certificates lifecycle projection', () => {
  it('separates assets from read-only automatic TLS observations', async () => {
    baseHandlers([]);
    const user = userEvent.setup();
    renderPage(<Certificates />, '/access/certificates');

    expect(await screen.findByText('*.example.com')).toBeInTheDocument();
    expect(screen.getByText(/自动 TLS 状态读取失败/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '上传证书' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /自动 TLS/ }));
    expect(await screen.findByText('auto.example.com')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '上传证书' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '申请证书' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    expect(screen.getByText('入口 TLS 策略')).toBeInTheDocument();
  });

  it('blocks referenced asset deletion and links to the owning route', async () => {
    baseHandlers([
      http.get('*/api/admin/v1/certificates/cert_wild/delete-preview', () => HttpResponse.json({
        action: 'delete_certificate', allowed: false, reason_code: 'CERT_HAS_REFERENCES',
        reason: 'certificate is still bound to routes', managed_by: 'user',
        references: [{ id: 'rt_api', domain: 'api.example.com' }], effects: [], alternatives: [],
      })),
    ]);
    const user = userEvent.setup();
    renderPage(<Certificates />);

    await user.click(await screen.findByRole('button', { name: '删除' }));
    expect(await screen.findByText('certificate is still bound to routes')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '确认删除' })).toBeDisabled();
    expect(screen.getByRole('link', { name: '处理绑定' })).toHaveAttribute('href', '/exposure/entry/rt_api');
  });

  it('previews and explicitly replaces automatic TLS in one bulk request', async () => {
    let submitted: unknown;
    baseHandlers([
      http.get('*/api/admin/v1/certificates/cert_wild/bindings', () => HttpResponse.json({
        cert_id: 'cert_wild', domains: '["*.example.com"]',
        candidates: [{
          route_id: 'rt_api', domain: 'api.example.com', current_binding_mode: 'provider_auto',
          already_bound: false, replaces_automatic_tls: true, selected: false,
        }],
      })),
      http.post('*/api/admin/v1/certificates/cert_wild/bindings', async ({ request }) => {
        submitted = await request.json();
        return HttpResponse.json({ status: 'updated' });
      }),
    ]);
    const user = userEvent.setup();
    renderPage(<Certificates />);

    await user.click(await screen.findByRole('button', { name: '批量绑定' }));
    expect(await screen.findByText('将替换自动 TLS')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '绑定所选域名 (1)' }));

    await waitFor(() => expect(submitted).toEqual({ route_ids: ['rt_api'] }));
    expect(await screen.findByText('证书已批量绑定')).toBeInTheDocument();
  });
});
