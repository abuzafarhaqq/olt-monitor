import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppLayout } from "../components/layout/AppLayout";
import { ALL_NAV_ITEMS } from "../constants/navigation";
import { OverviewPage } from "../features/dashboard/OverviewPage";
import { PlaceholderPage } from "../features/placeholder/PlaceholderPage";

/**
 * Declarative routing: routes are JSX, and every non-overview route is
 * generated from the same navigation list the sidebar draws from. Adding a
 * page means adding one entry to navigation.ts.
 */
export function AppRouter() {
  const secondaryRoutes = ALL_NAV_ITEMS.filter((item) => item.path !== "/");

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<OverviewPage />} />
          {secondaryRoutes.map((item) => (
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
