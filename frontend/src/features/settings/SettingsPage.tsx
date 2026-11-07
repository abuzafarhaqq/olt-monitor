import { PageHeader } from "../../components/common/PageHeader";
import { PermissionGate } from "../../components/common/PermissionGate";
import { listThresholds } from "../../constants/opticalThresholds";
import { DETAILED_RETENTION_DAYS } from "../../constants/reports";
import { HISTORY_RANGE } from "../../mock/history";
import { formatDateTime } from "../../utils/format";

const INTERVALS = [
  { id: "standard-5m", status: "ONU status + optics every 5 min", events: "syslog every 5 min", use: "default for reachable devices" },
  { id: "slow-15m", status: "ONU status every 15 min", events: "syslog every 15 min", use: "low-capacity or remote links" },
  { id: "manual", status: "no scheduled collection", events: "none", use: "devices under maintenance" },
];

const AUDIT = [
  { at: "2026-11-05T22:14:00Z", actor: "Administrator", change: "Optical threshold GPON: warning −26 → −27 dBm" },
  { at: "2026-11-04T09:02:00Z", actor: "Network Engineer", change: "Collection profile for GPON-OLT-03 set to slow-15m" },
  { at: "2026-11-02T18:40:00Z", actor: "Administrator", change: "Detailed measurement retention set to 90 days" },
];

export function SettingsPage() {
  const thresholds = listThresholds();

  return (
    <>
      <PageHeader
        title="Settings"
        subtitle="Read-only until the write API exists. Every change here will be audited server-side."
        isMock
      />

      <div className="callout" role="note">
        <strong>Model-specific thresholds.</strong> RX warning and critical levels are declared per
        vendor and technology. A pair that is not listed has no verdict — the UI shows
        <span className="badge badge--unknown" style={{ margin: "0 4px" }}>No threshold</span>
        rather than guessing one.
      </div>

      <section className="card" aria-label="Optical thresholds">
        <div className="card__header">
          <h2 className="card__title">Optical thresholds</h2>
          <span className="card__aside">{thresholds.length} calibrated pairs</span>
        </div>
        <div className="card__body card__body--flush">
          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">Vendor / technology</th>
                <th scope="col" className="align-right">Warning (dBm)</th>
                <th scope="col" className="align-right">Critical (dBm)</th>
                <th scope="col">State</th>
              </tr>
            </thead>
            <tbody>
              {thresholds.map((threshold) => (
                <tr key={threshold.pair}>
                  <td className="mono">{threshold.pair}</td>
                  <td className="align-right tabular">{threshold.warningDbm}</td>
                  <td className="align-right tabular">{threshold.criticalDbm}</td>
                  <td>
                    <span className="badge badge--online">calibrated</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card" aria-label="Collection intervals" style={{ marginTop: "var(--space-4)" }}>
        <div className="card__header">
          <h2 className="card__title">Collection intervals</h2>
          <span className="card__aside">per-device profile</span>
        </div>
        <div className="card__body card__body--flush">
          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">Profile</th>
                <th scope="col">Status collection</th>
                <th scope="col">Event collection</th>
                <th scope="col">Use</th>
              </tr>
            </thead>
            <tbody>
              {INTERVALS.map((interval) => (
                <tr key={interval.id}>
                  <td className="mono">{interval.id}</td>
                  <td>{interval.status}</td>
                  <td>{interval.events}</td>
                  <td className="cell-sub">{interval.use}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <div className="settings-grid">
        <section className="card" aria-label="Retention">
          <div className="card__header">
            <h2 className="card__title">Data retention</h2>
          </div>
          <div className="card__body">
            <dl className="facts facts--plain">
              <div className="fact">
                <dt className="fact__label">Detailed measurements</dt>
                <dd className="fact__value">{DETAILED_RETENTION_DAYS} days</dd>
              </div>
              <div className="fact">
                <dt className="fact__label">Daily aggregates</dt>
                <dd className="fact__value">{HISTORY_RANGE.days} days</dd>
              </div>
              <div className="fact">
                <dt className="fact__label">Available history</dt>
                <dd className="fact__value tabular">
                  {HISTORY_RANGE.from} → {HISTORY_RANGE.to}
                </dd>
              </div>
            </dl>
          </div>
        </section>

        <section className="card" aria-label="Event handling">
          <div className="card__header">
            <h2 className="card__title">Event &amp; incident handling</h2>
          </div>
          <div className="card__body">
            <dl className="facts facts--plain">
              <div className="fact">
                <dt className="fact__label">Deduplication</dt>
                <dd className="fact__value">off — raw events are preserved</dd>
              </div>
              <div className="fact">
                <dt className="fact__label">Out-of-order protection</dt>
                <dd className="fact__value">enabled — older state never overwrites newer</dd>
              </div>
              <div className="fact">
                <dt className="fact__label">Incident auto-create</dt>
                <dd className="fact__value">hypothesis only, never a verdict</dd>
              </div>
            </dl>
          </div>
        </section>
      </div>

      <section className="card" aria-label="Audit" style={{ marginTop: "var(--space-4)" }}>
        <div className="card__header">
          <h2 className="card__title">Audit trail</h2>
          <span className="card__aside">last {AUDIT.length} configuration changes</span>
        </div>
        <div className="card__body card__body--flush">
          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">Actor</th>
                <th scope="col">Change</th>
              </tr>
            </thead>
            <tbody>
              {AUDIT.map((entry) => (
                <tr key={entry.at}>
                  <td className="tabular">{formatDateTime(entry.at)}</td>
                  <td>{entry.actor}</td>
                  <td className="cell-sub">{entry.change}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <p className="page-footnote">
        <PermissionGate permission="settings.manage" fallback="Editing requires the Administrator role.">
          Editing arrives with the settings API (stage 7 backend).
        </PermissionGate>
      </p>
    </>
  );
}
