import { useState } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

/**
 * The shell every page renders inside.
 *
 * <Outlet /> is where the matched route's page is drawn, so pages never know
 * they are wrapped in a sidebar.
 */
export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false);

  return (
    <div className={navOpen ? "app-shell app-shell--nav-open" : "app-shell"}>
      {navOpen ? (
        <button
          type="button"
          className="nav-scrim"
          aria-label="Close navigation"
          onClick={() => setNavOpen(false)}
        />
      ) : null}

      <Sidebar onNavigate={() => setNavOpen(false)} />

      <div className="app-main">
        <Topbar navOpen={navOpen} onToggleNav={() => setNavOpen((open) => !open)} />
        <main className="app-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
