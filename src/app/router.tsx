import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppLayout } from "../components/layout/AppLayout";
import { ALL_NAV_ITEMS } from "../constants/navigation";
import { CollectionHealthPage } from "../features/collection/CollectionHealthPage";
import { OverviewPage } from "../features/dashboard/OverviewPage";
import { PlaceholderPage } from "../features/placeholder/PlaceholderPage";

/** Routes that exist for real. Everything else still gets a placeholder. */
const BUILT_PATHS = new Set(["/", "/collection"]);

/**
 * Declarative routing: routes are JSX, and every non-overview route is
 * generated from the same navigation list the sidebar draws from. Adding a
 * page means adding one entry to navigation.ts and one element here.
 */
export function AppRouter() {
  const placeholderRoutes = ALL_NAV_ITEMS.filter((item) => !BUILT_PATHS.has(item.path));

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<OverviewPage />} />
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
