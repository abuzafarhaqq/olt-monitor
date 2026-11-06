import { MOCK_OVERVIEW } from "../../mock/mockData";
import { formatRelative } from "../../utils/format";

interface TopbarProps {
  navOpen: boolean;
  onToggleNav: () => void;
}

export function Topbar({ navOpen, onToggleNav }: TopbarProps) {
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
          placeholder="Search OLT, ONU, MAC, incident… (stage 2)"
          // Searching live data before the API exists would be a lie.
          disabled
        />
      </div>

      <div className="topbar__meta">
        <span className="topbar__status">
          <span className="topbar__dot topbar__dot--mock" aria-hidden="true" />
          API: mock
        </span>
        <span className="topbar__status">
          <span className="topbar__dot" aria-hidden="true" />
          Live updates: off
        </span>
        <span className="topbar__status tabular">
          snapshot {formatRelative(MOCK_OVERVIEW.generatedAt)}
        </span>
        <span className="topbar__user">NOC Operator</span>
      </div>
    </header>
  );
}
