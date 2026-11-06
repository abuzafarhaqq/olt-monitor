import { useAuth } from "../../app/AuthContext";
import { useLiveEvents } from "../../hooks/useLiveEvents";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import { ROLES } from "../../services/auth";
import { formatRelative } from "../../utils/format";

interface TopbarProps {
  navOpen: boolean;
  onToggleNav: () => void;
}

export function Topbar({ navOpen, onToggleNav }: TopbarProps) {
  const { user, setRole } = useAuth();
  const liveEvents = useLiveEvents(5);
  const lastEvent = liveEvents[0];

  return (
    <header className="topbar">
      <button
        type="button"
        className="topbar__nav-toggle"
        onClick={onToggleNav}
        aria-expanded={navOpen}
        aria-label="Toggle navigation"
      >
        ☰
      </button>

      <div className="topbar__search">
        <label className="visually-hidden" htmlFor="global-search">
          Global search
        </label>
        <input
          id="global-search"
          type="search"
          placeholder="Search OLT, ONU, MAC, incident… (stage 8)"
          // Searching live data before the API exists would be a lie.
          disabled
        />
      </div>

      <div className="topbar__meta">
        <span className="topbar__status">
          <span className="topbar__dot topbar__dot--mock" aria-hidden="true" />
          API: mock
        </span>
        <span className="topbar__status" title={lastEvent?.message ?? "No live event yet"}>
          <span
            className={`topbar__dot ${liveEvents.length > 0 ? "topbar__dot--live" : ""}`}
            aria-hidden="true"
          />
          {liveEvents.length > 0 ? `live · ${liveEvents.length}` : "live: idle"}
        </span>
        <span className="topbar__status tabular">
          snapshot {formatRelative(MOCK_OVERVIEW.generatedAt)}
        </span>

        <label className="topbar__role">
          <span className="visually-hidden">Role</span>
          <select
            className="filter-field__control topbar__role-select"
            value={user.role}
            onChange={(event) => setRole(event.target.value as typeof user.role)}
          >
            {ROLES.map((role) => (
              <option key={role.id} value={role.id}>
                {role.label}
              </option>
            ))}
          </select>
        </label>
      </div>
    </header>
  );
}
