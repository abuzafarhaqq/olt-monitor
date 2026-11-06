import { NavLink } from "react-router-dom";
import { NAV_GROUPS } from "../../constants/navigation";

interface SidebarProps {
  /** Lets the drawer close itself after a click on small screens. */
  onNavigate?: () => void;
}

export function Sidebar({ onNavigate }: SidebarProps) {
  return (
    <aside className="sidebar" aria-label="Main navigation">
      <div className="sidebar__brand">
        <span className="sidebar__brand-mark" aria-hidden="true">
          OLT
        </span>
        <span className="sidebar__brand-text">
          <span className="sidebar__brand-title">OLT Diagnostics</span>
          <span className="sidebar__brand-subtitle">ISP NOC · Dhaka</span>
        </span>
      </div>

      <nav className="sidebar__nav">
        {NAV_GROUPS.map((group) => (
          <div className="sidebar__group" key={group.title}>
            <p className="sidebar__group-title">{group.title}</p>
            {group.items.map((item) => (
              <NavLink
                key={item.path}
                to={item.path}
                // `end` stops "/" from matching every route.
                end={item.path === "/"}
                onClick={onNavigate}
                className={({ isActive }) =>
                  isActive ? "sidebar__link sidebar__link--active" : "sidebar__link"
                }
              >
                <span>{item.label}</span>
                {item.stage > 1 ? (
                  <span className="sidebar__link-badge" title={`Planned for stage ${item.stage}`}>
                    S{item.stage}
                  </span>
                ) : null}
              </NavLink>
            ))}
          </div>
        ))}
      </nav>

      <div className="sidebar__footer">
        Stage 1 · mock data only
        <br />
        No connection to OLTs from the browser
      </div>
    </aside>
  );
}
