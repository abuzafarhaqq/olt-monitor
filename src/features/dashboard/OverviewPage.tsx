import { AvailabilityTrendChart } from "../../components/charts/AvailabilityTrendChart";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { ErrorState, LoadingState } from "../../components/common/States";
import { useOverview } from "../../hooks/useResource";
import { USE_MOCK_API } from "../../services/client";
import { formatRelative } from "../../utils/format";
import { OltHealthTable } from "./OltHealthTable";
import { ProblemPons } from "./ProblemPons";
import { RecentEvents } from "./RecentEvents";
import { getKpis, getRecentEvents, getTopProblemPons } from "./selectors";

/**
 * Stage 1 landing page. Every number on this screen is derived from
 * MOCK_OVERVIEW through a selector — nothing is hard-coded in the markup, so
 * Stage 2 only has to replace the data source, not the page.
 */
export function OverviewPage() {
  const { data, loading, error } = useOverview();

  if (loading) return <LoadingState label="Loading overview…" />;
  if (data === null) return <ErrorState description={error ?? "The overview returned no data."} />;

  const kpis = getKpis(data);
  const events = getRecentEvents(data.events, 6);
  const problems = getTopProblemPons(data, 4);

  return (
    <>
      <PageHeader
        title="Network Overview"
        subtitle={`Snapshot generated ${formatRelative(data.generatedAt)} · ${kpis.totalOlts} OLTs · ${kpis.totalOnus} ONUs`}
        isMock={USE_MOCK_API}
      />

      <section className="kpi-grid" aria-label="Key metrics">
        <MetricCard
          label="OLTs reachable"
          value={`${kpis.reachableOlts}/${kpis.totalOlts}`}
          hint={kpis.reachableOlts === kpis.totalOlts ? "all reachable" : `${kpis.totalOlts - kpis.reachableOlts} unreachable`}
          tone={kpis.reachableOlts === kpis.totalOlts ? "online" : "warning"}
        />
        <MetricCard label="Total ONUs" value={kpis.totalOnus} hint="across all OLTs" />
        <MetricCard label="Online" value={kpis.online} hint="authoritative status" tone="online" />
        <MetricCard label="Offline" value={kpis.offline} hint="confirmed by collection" tone="offline" />
        <MetricCard
          label="Unknown"
          value={kpis.unknown}
          hint="no trustworthy status — not offline"
          tone="unknown"
        />
        <MetricCard
          label="Active incidents"
          value={kpis.activeIncidents}
          hint="new, acknowledged, investigating"
          tone="investigating"
        />
        <MetricCard
          label="Unhealthy collections"
          value={kpis.unhealthyCollections}
          hint="OLTs whose last run failed or was partial"
          tone="warning"
        />
        <MetricCard
          label="Optical readings"
          value={data.onus.filter((onu) => onu.rxPowerDbm !== null).length}
          hint="missing readings are not faults"
          tone="info"
        />
      </section>

      <div className="dashboard-grid">
        <div className="dashboard-column">
          <section className="card" aria-label="ONU availability trend">
            <div className="card__header">
              <h2 className="card__title">ONU availability — last 24 hours</h2>
              <span className="card__aside">hourly buckets · online / offline / unknown</span>
            </div>
            <div className="card__body">
              <AvailabilityTrendChart points={data.availabilityTrend} />
            </div>
          </section>

          <section className="card" aria-label="OLT health">
            <div className="card__header">
              <h2 className="card__title">OLT health</h2>
              <span className="card__aside">device state and collection state are separate</span>
            </div>
            <div className="card__body card__body--flush">
              <OltHealthTable olts={data.olts} onus={data.onus} />
            </div>
          </section>
        </div>

        <div className="dashboard-column">
          <section className="card" aria-label="Recent events">
            <div className="card__header">
              <h2 className="card__title">Recent events</h2>
              <span className="card__aside">raw events · duplicates preserved</span>
            </div>
            <div className="card__body card__body--flush">
              <RecentEvents events={events} olts={data.olts} />
            </div>
          </section>

          <section className="card" aria-label="Top problem PONs">
            <div className="card__header">
              <h2 className="card__title">Top problem PONs</h2>
              <span className="card__aside">fresh data only</span>
            </div>
            <div className="card__body card__body--flush">
              <ProblemPons problems={problems} />
            </div>
          </section>
        </div>
      </div>
    </>
  );
}
