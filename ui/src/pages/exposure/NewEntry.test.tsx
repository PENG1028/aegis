import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import NewEntry from './NewEntry';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

const validAsset = {
  id: 'cert_match', domains: '["*.example.com"]', issuer: 'Test CA',
  not_before: '2026-01-01T00:00:00Z', not_after: '2030-01-01T00:00:00Z',
  source: 'manual_upload', record_type: 'asset', managed: true, managed_by: 'user',
  note: '', ref_count: 0, auto_renew: false, created_at: '2026-01-01T00:00:00Z',
};

function registerHandlers(availability: 'ready' | 'degraded' | 'unavailable') {
  server.use(
    http.get('*/api/system/runtime-mode', () => HttpResponse.json({
      current: {
        id: 'active', label: 'Active', providers: [{ provider_id: 'executor-a' }],
        compositions: [{ name: 'HTTPS Route', status: 'available', atoms: ['tcp', 'tls', 'http'], chain: 'tcp -> tls -> http' }],
      },
      available_modes: [],
    })),
    http.get('*/api/admin/v1/nodes', () => HttpResponse.json({ nodes: [] })),
    http.get('*/api/admin/v1/providers', () => HttpResponse.json({
      providers: [{
        id: 'executor-a', installed: true, running: availability === 'ready',
        capabilities: ['auto_cert'],
        capability_statuses: [{ key: 'auto_cert', availability, semantics: { state_class: 'provider_managed', affinity: 'executor_sticky', migration: 'recreate' } }],
      }],
    })),
    http.get('*/api/admin/v1/certificates', () => HttpResponse.json({
      certificates: [
        validAsset,
        { ...validAsset, id: 'cert_expired', domains: '["api.example.com"]', not_after: '2025-01-01T00:00:00Z' },
        { ...validAsset, id: 'cert_other', domains: '["other.example.net"]' },
        { ...validAsset, id: 'managed_auto', source: 'gateway_auto', record_type: 'provider_observation' },
      ],
      assets: [validAsset], automatic_tls: [],
    })),
  );
}

describe('NewEntry TLS capability selection', () => {
  it('enables ready automatic TLS and only recommends a matching asset', async () => {
    registerHandlers('ready');
    const user = userEvent.setup();
    renderPage(<NewEntry />, '/exposure/new');

    const automaticTLS = await screen.findByRole('button', { name: '自动 TLS' });
    expect(automaticTLS).toBeEnabled();
    await user.type(screen.getByPlaceholderText('api.example.com'), 'api.example.com');

    expect(await screen.findByText(/发现 1 张覆盖当前域名的证书资产/)).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /\*\.example\.com/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '手动指定' }));
    expect(screen.getByRole('option', { name: /\*\.example\.com/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /other\.example\.net/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /cert_expired/ })).not.toBeInTheDocument();
  });

  it.each(['degraded', 'unavailable'] as const)('disables %s automatic TLS and shows the capability reason', async availability => {
    registerHandlers(availability);
    renderPage(<NewEntry />, '/exposure/new');

    expect(await screen.findByRole('button', { name: '自动 TLS — 不可用' })).toBeDisabled();
    expect(screen.getByText(/自动 TLS 能力未就绪/)).toBeInTheDocument();
  });
});
