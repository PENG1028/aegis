import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProvidersDetail from './ProvidersDetail';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

// The reload endpoint answers 200 for a *valid request* and reports the outcome in
// the body — see internal/httpapi/handlers/provider_control.go, whose contract is
// pinned in provider_control_test.go. The API client only rejects on a non-2xx
// status, so the body is the only place a failed reload is visible to the UI.
function registerProvider(reloadBody: Record<string, unknown>, reloadStatus = 200) {
  server.use(
    http.get('*/api/admin/v1/providers', () => HttpResponse.json({
      providers: [{
        id: 'caddy', name: 'Caddy', status: 'ready', installed: true, running: true,
        capabilities: ['route_host'], theoretical_capabilities: [], issues: [],
      }],
      count: 1,
      capability_universe: [{ key: 'route_host', label: 'Route Host', layer: 'L7' }],
    })),
    http.post('*/api/admin/v1/providers/caddy/reload', () =>
      HttpResponse.json(reloadBody, { status: reloadStatus })),
  );
}

// The reload button lives inside a collapsed card, so the header has to be opened
// first. "Caddy" also appears outside the card, so pick the occurrence wrapped in
// the header button.
async function openReloadButton(user: ReturnType<typeof userEvent.setup>) {
  const labels = await screen.findAllByText('Caddy');
  const header = labels.map(el => el.closest('button')).find(Boolean);
  if (!header) throw new Error('no provider card header button found for Caddy');
  await user.click(header);
  return screen.findByRole('button', { name: '热重载' });
}

// A reload the gateway refused must not be reported as succeeded. The operator
// clicked reload to make new config live; being told it worked means they stop
// there, while the gateway keeps serving the old configuration.
it('reports a refused reload as a failure, not a success', async () => {
  registerProvider({
    provider: 'caddy', action: 'reload', status: 'failed',
    error: 'config validation failed at line 12',
  });
  const user = userEvent.setup();
  renderPage(<ProvidersDetail />);

  await user.click(await openReloadButton(user));

  expect(await screen.findByText(/失败|line 12/)).toBeInTheDocument();
  expect(screen.queryByText(/重载成功/)).not.toBeInTheDocument();
});

// The other direction, so a UI that always reported failure would not pass.
it('reports a completed reload as a success', async () => {
  registerProvider({ provider: 'caddy', action: 'reload', status: 'success' });
  const user = userEvent.setup();
  renderPage(<ProvidersDetail />);

  await user.click(await openReloadButton(user));

  expect(await screen.findByText(/重载成功/)).toBeInTheDocument();
});

// A transport-level failure already rejects the promise. This pins that the error
// path still surfaces it, so the fix for the body case cannot regress it.
it('surfaces a transport failure', async () => {
  registerProvider({ error: 'boom' }, 500);
  const user = userEvent.setup();
  renderPage(<ProvidersDetail />);

  await user.click(await openReloadButton(user));

  expect(await screen.findByText(/失败/)).toBeInTheDocument();
  expect(screen.queryByText(/重载成功/)).not.toBeInTheDocument();
});
