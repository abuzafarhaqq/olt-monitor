import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppLayout } from "../components/layout/AppLayout";
import { RequireAuth } from "./RequireAuth";
import { ForgotPasswordPage } from "../features/auth/ForgotPasswordPage";
import { LoginPage } from "../features/auth/LoginPage";
import { ALL_NAV_ITEMS } from "../constants/navigation";
import { CollectionHealthPage } from "../features/collection/CollectionHealthPage";
import { DiagnosticsConsolePage } from "../features/console/DiagnosticsConsolePage";
import { OverviewPage } from "../features/dashboard/OverviewPage";
import { IncidentsPage } from "../features/incidents/IncidentsPage";
import { OltDetailPage } from "../features/olts/OltDetailPage";
import { OltFormPage } from "../features/olts/OltFormPage";
import { OltInventoryPage } from "../features/olts/OltInventoryPage";
import { OpticalDiagnosticsPage } from "../features/optical/OpticalDiagnosticsPage";
import { OnuDetailPage } from "../features/onus/OnuDetailPage";
import { OnuListPage } from "../features/onus/OnuListPage";
import { PlaceholderPage } from "../features/placeholder/PlaceholderPage";
import { PonExplorerPage } from "../features/pons/PonExplorerPage";
import { ReportsPage } from "../features/reports/ReportsPage";
import { SettingsPage } from "../features/settings/SettingsPage";
import { UsersAccessPage } from "../features/users/UsersAccessPage";

/** Nav paths that already render a real page. */
const BUILT_PATHS = new Set([
  "/",
  "/olts",
  "/pons",
  "/onus",
  "/optical",
  "/incidents",
  "/reports",
  "/collection",
  "/console",
  "/users",
  "/settings",
]);

/**
 * Declarative routing: routes are JSX. Any nav entry that is not built yet
 * still gets a placeholder, generated from the same list the sidebar draws.
 */
export function AppRouter() {
  const placeholderRoutes = ALL_NAV_ITEMS.filter((item) => !BUILT_PATHS.has(item.path));

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        >
          <Route index element={<OverviewPage />} />
          <Route path="/olts" element={<OltInventoryPage />} />
          <Route path="/olts/new" element={<OltFormPage />} />
          <Route path="/olts/:oltId/edit" element={<OltFormPage />} />
          <Route path="/olts/:oltId" element={<OltDetailPage />} />
          <Route path="/pons" element={<PonExplorerPage />} />
          <Route path="/onus" element={<OnuListPage />} />
          <Route path="/onus/:onuId" element={<OnuDetailPage />} />
          <Route path="/optical" element={<OpticalDiagnosticsPage />} />
          <Route path="/incidents" element={<IncidentsPage />} />
          <Route path="/reports" element={<ReportsPage />} />
          <Route path="/collection" element={<CollectionHealthPage />} />
          <Route path="/console" element={<DiagnosticsConsolePage />} />
          <Route path="/users" element={<UsersAccessPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          {placeholderRoutes.map((item) => (
            <Route
              key={item.path}
              path={item.path}
              element={<PlaceholderPage title={item.label} stage={item.stage} />}
            />
          ))}
          <Route path="*" element={<PlaceholderPage title="Page not found" stage={0} />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
