import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AppLayout } from '@/components/layout/AppLayout';
import { AuthGuard } from '@/components/shared';

// Existing pages
import DashboardPage from '@/pages/Dashboard';
import NodesPage from '@/pages/Nodes';
import NodeDetailPage from '@/pages/NodeDetail';
import JoinTokensPage from '@/pages/JoinTokens';
import GatewaysPage from '@/pages/Gateways';
import GatewayDetailPage from '@/pages/GatewayDetail';
import TopologyPage from '@/pages/Topology';
import TopologyPathPage from '@/pages/TopologyPath';
import ServicesPage from '@/pages/Services';
import ServiceDetailPage from '@/pages/ServiceDetail';
import RoutesPage from '@/pages/Routes';
import RouteDetailPage from '@/pages/RouteDetail';
import EndpointsPage from '@/pages/Endpoints';
import GatewayPoliciesPage from '@/pages/GatewayPolicies';
import RoutingTablePage from '@/pages/RoutingTable';
import SyncStatusPage from '@/pages/SyncStatus';
import LocalGatewayRuntimePage from '@/pages/LocalGatewayRuntime';
import AcceptancePage from '@/pages/Acceptance';
import SettingsPage from '@/pages/Settings';

// New v2 pages
import TracePage from '@/pages/Trace';
import SafetyPage from '@/pages/Safety';
import RelayPage from '@/pages/Relay';
import GatewayLinksPage from '@/pages/GatewayLinksPage';
import GatewayLinkDetailPage from '@/pages/GatewayLinkDetailPage';
import { ApplyConfigPage } from '@/pages/ApplyConfig';
import { ConfigPage } from '@/pages/ConfigPage';
import ProvidersPage from '@/pages/ProvidersPage';
import ListenersPage from '@/pages/ListenersPage';
import { DoctorPage } from '@/pages/DoctorPage';
import { SmokePage } from '@/pages/SmokePage';
import ScopesPage from '@/pages/ScopesPage';
import ApiKeysPage from '@/pages/ApiKeysPage';
import LogsPage from '@/pages/LogsAudit';
import { SecurityPage } from '@/pages/SecurityPage';
import { MaintenancePage } from '@/pages/MaintenancePage';
import { ActionsPage } from '@/pages/ActionsPage';
import QuickCreatePage from '@/pages/QuickCreate';
import HealthCheckPage from '@/pages/HealthCheck';
import ImportConfigPage from '@/pages/ImportConfig';
import TransparentProxyPage from '@/pages/TransparentProxy';
import ExposuresPage from '@/pages/Exposures';
import CredentialsPage from '@/pages/Credentials';
import MiddlewarePage from '@/pages/Middleware';
import NotFoundPage from '@/pages/NotFound';

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
      <BrowserRouter>
        <AuthGuard>
          <Routes>
            <Route element={<AppLayout />}>
              {/* Dashboard */}
              <Route path="/" element={<DashboardPage />} />

              {/* Quick Create & Health */}
              <Route path="/quick-create" element={<QuickCreatePage />} />
              <Route path="/health" element={<HealthCheckPage />} />
              <Route path="/import" element={<ImportConfigPage />} />

              {/* Nodes & Join Tokens */}
              <Route path="/nodes" element={<NodesPage />} />
              <Route path="/nodes/:nodeId" element={<NodeDetailPage />} />
              <Route path="/join-tokens" element={<JoinTokensPage />} />

              {/* Gateways */}
              <Route path="/gateways" element={<GatewaysPage />} />
              <Route path="/gateways/:gatewayId" element={<GatewayDetailPage />} />

              {/* Topology */}
              <Route path="/topology" element={<TopologyPage />} />
              <Route path="/topology/path" element={<TopologyPathPage />} />

              {/* Services */}
              <Route path="/services" element={<ServicesPage />} />
              <Route path="/services/:serviceId" element={<ServiceDetailPage />} />

              {/* Routes */}
              <Route path="/routes" element={<RoutesPage />} />
              <Route path="/routes/:routeId" element={<RouteDetailPage />} />

              {/* Endpoints */}
              <Route path="/endpoints" element={<EndpointsPage />} />

              {/* Policies & Routing */}
              <Route path="/policies" element={<GatewayPoliciesPage />} />
              <Route path="/routing" element={<RoutingTablePage />} />

              {/* Sync & Local Gateway */}
              <Route path="/sync" element={<SyncStatusPage />} />
              <Route path="/local-gateway" element={<LocalGatewayRuntimePage />} />

              {/* Acceptance & Settings */}
              <Route path="/acceptance" element={<AcceptancePage />} />
              <Route path="/settings" element={<SettingsPage />} />

              {/* === New v2 pages === */}

              {/* Trace / Safety / Relay / Exposures */}
              <Route path="/trace" element={<TracePage />} />
              <Route path="/transparent" element={<TransparentProxyPage />} />
              <Route path="/exposures" element={<ExposuresPage />} />
              <Route path="/credentials" element={<CredentialsPage />} />
              <Route path="/safety" element={<SafetyPage />} />
              <Route path="/relay" element={<RelayPage />} />

              {/* Gateway Links */}
              <Route path="/gateway-links" element={<GatewayLinksPage />} />
              <Route path="/gateway-links/:id" element={<GatewayLinkDetailPage />} />

              {/* Apply / Config */}
              <Route path="/apply" element={<ApplyConfigPage />} />
              <Route path="/config" element={<ConfigPage />} />

              {/* Providers / Listeners */}
              <Route path="/providers" element={<ProvidersPage />} />
              <Route path="/middleware" element={<MiddlewarePage />} />
              <Route path="/listeners" element={<ListenersPage />} />

              {/* Doctor / Smoke */}
              <Route path="/doctor" element={<DoctorPage />} />
              <Route path="/smoke" element={<SmokePage />} />

              {/* Scopes / API Keys */}
              <Route path="/scopes" element={<ScopesPage />} />
              <Route path="/api-keys" element={<ApiKeysPage />} />

              {/* Logs */}
              <Route path="/logs" element={<LogsPage />} />
              <Route path="/audit" element={<LogsPage />} />

              {/* Security / Maintenance / Actions */}
              <Route path="/security" element={<SecurityPage />} />
              <Route path="/maintenance" element={<MaintenancePage />} />
              <Route path="/actions" element={<ActionsPage />} />

              {/* 404 */}
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Routes>
        </AuthGuard>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
