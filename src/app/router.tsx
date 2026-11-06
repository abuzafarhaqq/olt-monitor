import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppLayout } from "../components/layout/AppLayout";
import { ALL_NAV_ITEMS } from "../constants/navigation";
import { CollectionHealthPage } from "../features/collection/CollectionHealthPage";
import { OverviewPage } from "../features/dashboard/OverviewPage";
import { OltDetailPage } from "../features/olts/OltDetailPage";
import { OltInventoryPage } from "../features/olts/OltInventoryPage";
import { PlaceholderPage } from "../features/placeholder/PlaceholderPage";
import { PonExplorerPage } from "../features/pons/PonExplorerPage";

/** Nav paths that already render a real page. */
const BUILT_PATHS = new Set(["/", "/olts", "/pons", "/collection"]);

/**
 * Declarative routing: routes are JSX, and every remaining route is generated
 * from the same navigation list the sidebar draws from.
 */
export function AppRouter() {
  const placeholderRoutes = ALL_NAV_ITEMS.filter((item) => !BUILT_PATHS.has(item.path));

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<OverviewPage />} />
          <Route path="/olts" element={<OltInventoryPage />} />
          <Route path="/olts/:oltId" element={<OltDetailPage />} />
          <Route path="/pons" element={<PonExplorerPage />} />
          <Route path="/collection" element={<CollectionHealthPage />} />
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
