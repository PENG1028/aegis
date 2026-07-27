import { Outlet } from 'react-router-dom';

// OutletLayout is a minimal layout wrapper used by workspaces that
// don't yet need workspace-specific chrome (sidebar, tabs, etc.).
// It exists as a single shared component instead of 7 identical files.
export default function OutletLayout() {
  return <Outlet />;
}
