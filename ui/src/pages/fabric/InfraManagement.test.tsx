import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import InfraManagement from './InfraManagement';
import { renderPage } from '@/test/render';
import { server } from '@/test/server';

// The reload button here calls /providers/{id}/reload with a raw fetch. Two things
// make that easy to get wrong: the endpoint answers 200 for a valid request and
// reports the outcome in the body, and fetch() only rejects on a network error, so
// a 4xx/5xx resolves too. Both have to be checked or a failed reload reads as done.
function registerInfra(reloadBody: Record<string, unknown>, reloadStatus = 200) {
  server.use(
    http.get('*/api/admin/v1/providers', () => HttpResponse.json({
      providers: [{
        id: 'caddy', name: 'Caddy', status: 'ready', installed: true, running: true,
        capabilities: ['hot_reload'], theoretical_capabilities: [], issues: [],
      }],
      count: 1,
      capability_universe: [],
    })),
    http.get('*/api/admin/v1/infra/status', () => HttpResponse.json({ items: [] })),
    http.post('*/api/admin/v1/providers/caddy/reload', () =>
      HttpResponse.json(reloadBody, { status: reloadStatus })),
  );
}

const clickReload = async (user: ReturnType<typeof userEvent.setup>) =>
  user.click(await screen.findByRole('button', { name: '重载' }));

// A reload the gateway refused must not read as done. The operator clicked reload
// to make new config live; "已重载" means they stop there while the old config is
// still being served.
it('reports a refused reload as a failure', async () => {
  registerInfra({ provider: 'caddy', action: 'reload', status: 'failed', error: 'validation failed' });
  const user = userEvent.setup();
  renderPage(<InfraManagement />);

  await clickReload(user);

  expect(await screen.findByText(/重载失败/)).toBeInTheDocument();
  expect(screen.queryByText(/^已重载$/)).not.toBeInTheDocument();
});

// fetch() resolves on a 500, so without an explicit ok check this path reported
// success for a request the server never processed.
it('reports a server error as a failure', async () => {
  registerInfra({ error: 'internal' }, 500);
  const user = userEvent.setup();
  renderPage(<InfraManagement />);

  await clickReload(user);

  expect(await screen.findByText(/重载失败/)).toBeInTheDocument();
  expect(screen.queryByText(/^已重载$/)).not.toBeInTheDocument();
});

// An expired session is the other status that fetch() hands back as a resolved
// promise. Reporting it as reloaded hides the fact that nothing happened.
it('reports an unauthorized reload as a failure', async () => {
  registerInfra({ error: 'unauthorized' }, 401);
  const user = userEvent.setup();
  renderPage(<InfraManagement />);

  await clickReload(user);

  expect(await screen.findByText(/重载失败/)).toBeInTheDocument();
});

// The other direction, so a button that always reported failure would not pass.
it('reports a completed reload as done', async () => {
  registerInfra({ provider: 'caddy', action: 'reload', status: 'success' });
  const user = userEvent.setup();
  renderPage(<InfraManagement />);

  await clickReload(user);

  expect(await screen.findByText(/已重载/)).toBeInTheDocument();
});
