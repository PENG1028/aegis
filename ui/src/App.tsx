// ─── Aegis Frontend v2 ───
// Workspace-based routing: 8 workspaces × nested routes
// Relationship-driven UI with PathRibbon, RelationshipMap, ImpactPanel, ReleaseDiffViewer

import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthGuard, ToastProvider } from '@/components/shared';
import { AppShell } from '@/components/layout/AppShell';
import { ViewProvider } from '@/lib/view-context';
import { LEGACY_REDIRECTS } from '@/lib/constants';

// ── Command Center ──
import CommandCenter from '@/pages/command-center/CommandCenter';

// ── Shared layout for workspaces without custom chrome ──
import OutletLayout from '@/components/layout/OutletLayout';

// ── Exposure / 流量管理 ──
import EntryList from '@/pages/exposure/EntryList';
import NewEntry from '@/pages/exposure/NewEntry';
import EntryPointDetail from '@/pages/exposure/EntryPointDetail';

// ── Fabric ──
import Providers from '@/pages/fabric/Providers';
import ProvidersDetail from '@/pages/fabric/ProvidersDetail';
import EgressGateway from '@/pages/fabric/EgressGateway';
import ModeSwitch from '@/pages/fabric/ModeSwitch';
import GatewayServicePanel from '@/pages/fabric/GatewayServicePanel';
import Gateways from '@/pages/fabric/Gateways';
import GatewayDetail from '@/pages/fabric/GatewayDetail';
import GatewayLinks from '@/pages/fabric/GatewayLinks';
import Listeners from '@/pages/fabric/Listeners';
import Topology from '@/pages/fabric/Topology';
import RoutingTable from '@/pages/fabric/RoutingTable';

// ── Service Auth ──
import AuthServices from '@/pages/fabric/AuthServices';
import AuthCallGraph from '@/pages/fabric/AuthCallGraph';

// ── Runtime ──
import Nodes from '@/pages/runtime/Nodes';
import NodeDetail from '@/pages/runtime/NodeDetail';
import ClusterHealth from '@/pages/runtime/ClusterHealth';
import DeployNode from '@/pages/runtime/DeployNode';
import Updates from '@/pages/runtime/Updates';
import SyncStatus from '@/pages/runtime/SyncStatus';

// ── Release ──
import Changes from '@/pages/release/Changes';
import DiffView from '@/pages/release/DiffView';
import DryRun from '@/pages/release/DryRun';
import Apply from '@/pages/release/Apply';
import History from '@/pages/release/History';
import Rollback from '@/pages/release/Rollback';

// ── Observe ──
import Trace from '@/pages/observe/Trace';
import Health from '@/pages/observe/Health';
import Safety from '@/pages/observe/Safety';
import Logs from '@/pages/observe/Logs';
import Doctor from '@/pages/observe/Doctor';
import Acceptance from '@/pages/observe/Acceptance';

// ── Access ──
import Credentials from '@/pages/access/Credentials';
import JoinTokens from '@/pages/access/JoinTokens';
import AdminAccount from '@/pages/access/AdminAccount';
import Certificates from '@/pages/access/Certificates';

// ── Settings ──
import PanelSettings from '@/pages/settings/Panel';
import DnsSettings from '@/pages/settings/DnsSettings';
import TlsSettings from '@/pages/settings/TlsSettings';
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
    return <Navigate to="/fabric" replace />;
  }
  if (location.pathname.startsWith('/routes/')) {
    return <Navigate to={location.pathname.replace('/routes/', '/exposure/entry/')} replace />;
  }
  if (location.pathname.startsWith('/services/')) {
    return <Navigate to={location.pathname.replace('/services/', '/exposure/service/')} replace />;
  }
  if (location.pathname.startsWith('/gateway-links')) {
    return <Navigate to="/fabric" replace />;
  }
  if (location.pathname.startsWith('/settings/dns')) {
    return <Navigate to="/fabric/egress" replace />;
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
            <ViewProvider>
            <Routes>
              <Route element={<AppShell />}>
                {/* ── Workspace 1: Command Center ── */}
                <Route path="/" element={<CommandCenter />} />

                {/* ── Workspace 2: 流量管理 ── */}
                <Route path="/exposure" element={<OutletLayout />}>
                  <Route index element={<EntryList />} />
                  <Route path="new" element={<NewEntry />} />
                  <Route path="entry/:entryId" element={<EntryPointDetail />} />
                  <Route path="service/:serviceId" element={<Navigate to="/exposure" replace />} />
                </Route>

                {/* ── Workspace 3: Fabric / 网关网络 ── */}
                <Route path="/fabric" element={<OutletLayout />}>
                  <Route index element={<Providers />} />
                  <Route path="provider/:providerId" element={<ProvidersDetail />} />
                  <Route path="egress" element={<EgressGateway />} />
                  <Route path="mode" element={<ModeSwitch />} />
                  <Route path="service" element={<GatewayServicePanel />} />
                  <Route path="gateways" element={<Gateways />} />
                  <Route path="gateway/:gatewayId" element={<GatewayDetail />} />
                  <Route path="links" element={<GatewayLinks />} />
                  <Route path="listeners" element={<Listeners />} />
                  <Route path="topology" element={<Topology />} />
                  <Route path="routing-table" element={<RoutingTable />} />
                </Route>

                {/* ── Workspace 3b: Service Auth / 服务认证 ── */}
                <Route path="/auth" element={<OutletLayout />}>
                  <Route index element={<AuthServices />} />
                  <Route path="callgraph" element={<AuthCallGraph />} />
                </Route>

                {/* ── Workspace 4: Runtime / 节点运行时 ── */}
                <Route path="/runtime" element={<OutletLayout />}>
                  <Route index element={<Nodes />} />
                  <Route path="node/:nodeId" element={<NodeDetail />} />
                  <Route path="cluster" element={<ClusterHealth />} />
                  <Route path="deploy" element={<DeployNode />} />
                  <Route path="updates" element={<Updates />} />
                  <Route path="sync" element={<SyncStatus />} />
                </Route>

                {/* ── Workspace 5: Release / 配置发布 ── */}
                <Route path="/release" element={<OutletLayout />}>
                  <Route index element={<Changes />} />
                  <Route path="diff" element={<DiffView />} />
                  <Route path="dry-run" element={<DryRun />} />
                  <Route path="apply" element={<Apply />} />
                  <Route path="history" element={<History />} />
                  <Route path="rollback" element={<Rollback />} />
                </Route>

                {/* ── Workspace 6: Observe / 观测诊断 ── */}
                <Route path="/observe" element={<OutletLayout />}>
                  <Route index element={<Trace />} />
                  <Route path="health" element={<Health />} />
                  <Route path="safety" element={<Safety />} />
                  <Route path="logs" element={<Logs />} />
                  <Route path="audit" element={<Logs />} />
                  <Route path="doctor" element={<Doctor />} />
                  <Route path="acceptance" element={<Acceptance />} />
                </Route>

                {/* ── Workspace 7: Access / 访问控制 ── */}
                <Route path="/access" element={<OutletLayout />}>
                  <Route index element={<Navigate to="/access/admin" replace />} />
                  <Route path="credentials" element={<Credentials />} />
                  <Route path="certificates" element={<Certificates />} />
                  <Route path="tokens" element={<JoinTokens />} />
                  <Route path="admin" element={<AdminAccount />} />
                </Route>

                {/* ── Workspace 8: Settings / 系统设置 ── */}
                <Route path="/settings" element={<OutletLayout />}>
                  <Route index element={<PanelSettings />} />
                  <Route path="dns" element={<DnsSettings />} />
                  <Route path="tls" element={<TlsSettings />} />
                  <Route path="advanced" element={<AdvancedSettings />} />
                </Route>

                {/* ── Legacy redirects + 404 ── */}
                <Route path="*" element={<LegacyRedirect />} />
              </Route>
            </Routes>
            </ViewProvider>
          </AuthGuard>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  );
}
