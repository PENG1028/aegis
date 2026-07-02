// ─── Aegis Frontend v2 ───
// Workspace-based routing: 8 workspaces × nested routes
// Relationship-driven UI with PathRibbon, RelationshipMap, ImpactPanel, ReleaseDiffViewer

import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthGuard, ToastProvider } from '@/components/shared';
import { AppShell } from '@/components/layout/AppShell';
import { LEGACY_REDIRECTS } from '@/lib/constants';

// ── Command Center ──
import CommandCenter from '@/pages/command-center/CommandCenter';

// ── Exposure ──
import ExposureLayout from '@/pages/exposure/ExposureLayout';
import EntryPoints from '@/pages/exposure/EntryPoints';
import EntryPointDetail from '@/pages/exposure/EntryPointDetail';
import Services from '@/pages/exposure/Services';
import ServiceDetail from '@/pages/exposure/ServiceDetail';
import Endpoints from '@/pages/exposure/Endpoints';
import QuickConnect from '@/pages/exposure/QuickConnect';
import ImportConfig from '@/pages/exposure/ImportConfig';

// ── Fabric ──
import FabricLayout from '@/pages/fabric/FabricLayout';
import Gateways from '@/pages/fabric/Gateways';
import GatewayDetail from '@/pages/fabric/GatewayDetail';
import Listeners from '@/pages/fabric/Listeners';
import GatewayLinks from '@/pages/fabric/GatewayLinks';
import Topology from '@/pages/fabric/Topology';
import RoutingTable from '@/pages/fabric/RoutingTable';
import Providers from '@/pages/fabric/Providers';

// ── Runtime ──
import RuntimeLayout from '@/pages/runtime/RuntimeLayout';
import Nodes from '@/pages/runtime/Nodes';
import NodeDetail from '@/pages/runtime/NodeDetail';
import DeployNode from '@/pages/runtime/DeployNode';
import Updates from '@/pages/runtime/Updates';
import SyncStatus from '@/pages/runtime/SyncStatus';
import Binaries from '@/pages/runtime/Binaries';

// ── Release ──
import ReleaseLayout from '@/pages/release/ReleaseLayout';
import Changes from '@/pages/release/Changes';
import DiffView from '@/pages/release/DiffView';
import DryRun from '@/pages/release/DryRun';
import Apply from '@/pages/release/Apply';
import History from '@/pages/release/History';
import Rollback from '@/pages/release/Rollback';

// ── Observe ──
import ObserveLayout from '@/pages/observe/ObserveLayout';
import Trace from '@/pages/observe/Trace';
import Health from '@/pages/observe/Health';
import Safety from '@/pages/observe/Safety';
import Logs from '@/pages/observe/Logs';
import Doctor from '@/pages/observe/Doctor';
import Acceptance from '@/pages/observe/Acceptance';

// ── Access ──
import AccessLayout from '@/pages/access/AccessLayout';
import Scopes from '@/pages/access/Scopes';
import ApiKeys from '@/pages/access/ApiKeys';
import Credentials from '@/pages/access/Credentials';
import JoinTokens from '@/pages/access/JoinTokens';
import AdminAccount from '@/pages/access/AdminAccount';

// ── Settings ──
import SettingsLayout from '@/pages/settings/SettingsLayout';
import PanelSettings from '@/pages/settings/Panel';
import DnsSettings from '@/pages/settings/DnsSettings';
import TlsSettings from '@/pages/settings/TlsSettings';
import TransparentProxyPage from '@/pages/settings/TransparentProxy';
import AdvancedSettings from '@/pages/settings/Advanced';

// ── Legacy ──
import NotFound from '@/pages/NotFound';

// ── Legacy Redirect Component ──
function LegacyRedirect() {
  const location = useLocation();
  const target = LEGACY_REDIRECTS[location.pathname];
  if (target) {
    return <Navigate to={target} replace />;
  }
  // Handle nested paths like /nodes/:id, /gateways/:id, /routes/:id, /services/:id, /gateway-links/:id
  if (location.pathname.startsWith('/nodes/')) {
    return <Navigate to={location.pathname.replace('/nodes/', '/runtime/node/')} replace />;
  }
  if (location.pathname.startsWith('/gateways/')) {
    return <Navigate to={location.pathname.replace('/gateways/', '/fabric/gateway/')} replace />;
  }
  if (location.pathname.startsWith('/routes/')) {
    return <Navigate to={location.pathname.replace('/routes/', '/exposure/entry/')} replace />;
  }
  if (location.pathname.startsWith('/services/')) {
    return <Navigate to={location.pathname.replace('/services/', '/exposure/service/')} replace />;
  }
  if (location.pathname.startsWith('/gateway-links')) {
    return <Navigate to="/fabric/links" replace />;
  }
  if (location.pathname.startsWith('/topology/path')) {
    return <Navigate to="/fabric/topology" replace />;
  }
  return <NotFound />;
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30_000,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <BrowserRouter>
          <AuthGuard>
            <Routes>
              <Route element={<AppShell />}>
                {/* ── Workspace 1: Command Center ── */}
                <Route path="/" element={<CommandCenter />} />

                {/* ── Workspace 2: Exposure / 服务暴露 ── */}
                <Route path="/exposure" element={<ExposureLayout />}>
                  <Route index element={<EntryPoints />} />
                  <Route path="entry/:entryId" element={<EntryPointDetail />} />
                  <Route path="service/:serviceId" element={<ServiceDetail />} />
                  <Route path="endpoint/:endpointId" element={<EntryPointDetail />} />
                  <Route path="connect" element={<QuickConnect />} />
                  <Route path="import" element={<ImportConfig />} />
                </Route>

                {/* ── Workspace 3: Fabric / 网关网络 ── */}
                <Route path="/fabric" element={<FabricLayout />}>
                  <Route index element={<Gateways />} />
                  <Route path="gateway/:gatewayId" element={<GatewayDetail />} />
                  <Route path="listeners" element={<Listeners />} />
                  <Route path="links" element={<GatewayLinks />} />
                  <Route path="topology" element={<Topology />} />
                  <Route path="routing" element={<RoutingTable />} />
                  <Route path="providers" element={<Providers />} />
                  <Route path="providers/:providerId" element={<Providers />} />
                </Route>

                {/* ── Workspace 4: Runtime / 节点运行时 ── */}
                <Route path="/runtime" element={<RuntimeLayout />}>
                  <Route index element={<Nodes />} />
                  <Route path="node/:nodeId" element={<NodeDetail />} />
                  <Route path="deploy" element={<DeployNode />} />
                  <Route path="updates" element={<Updates />} />
                  <Route path="sync" element={<SyncStatus />} />
                  <Route path="binaries" element={<Binaries />} />
                </Route>

                {/* ── Workspace 5: Release / 配置发布 ── */}
                <Route path="/release" element={<ReleaseLayout />}>
                  <Route index element={<Changes />} />
                  <Route path="diff" element={<DiffView />} />
                  <Route path="dry-run" element={<DryRun />} />
                  <Route path="apply" element={<Apply />} />
                  <Route path="history" element={<History />} />
                  <Route path="rollback" element={<Rollback />} />
                </Route>

                {/* ── Workspace 6: Observe / 观测诊断 ── */}
                <Route path="/observe" element={<ObserveLayout />}>
                  <Route index element={<Trace />} />
                  <Route path="health" element={<Health />} />
                  <Route path="safety" element={<Safety />} />
                  <Route path="logs" element={<Logs />} />
                  <Route path="audit" element={<Logs />} />
                  <Route path="doctor" element={<Doctor />} />
                  <Route path="acceptance" element={<Acceptance />} />
                </Route>

                {/* ── Workspace 7: Access / 访问控制 ── */}
                <Route path="/access" element={<AccessLayout />}>
                  <Route index element={<Scopes />} />
                  <Route path="keys" element={<ApiKeys />} />
                  <Route path="credentials" element={<Credentials />} />
                  <Route path="tokens" element={<JoinTokens />} />
                  <Route path="admin" element={<AdminAccount />} />
                </Route>

                {/* ── Workspace 8: Settings / 系统设置 ── */}
                <Route path="/settings" element={<SettingsLayout />}>
                  <Route index element={<PanelSettings />} />
                  <Route path="dns" element={<DnsSettings />} />
                  <Route path="tls" element={<TlsSettings />} />
                  <Route path="proxy" element={<TransparentProxyPage />} />
                  <Route path="advanced" element={<AdvancedSettings />} />
                </Route>

                {/* ── Legacy redirects + 404 ── */}
                <Route path="*" element={<LegacyRedirect />} />
              </Route>
            </Routes>
          </AuthGuard>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  );
}
