import { useState, type ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { CollectionBadge } from "../../components/common/CollectionBadge";
import { DataTable, type Column } from "../../components/common/DataTable";
import { EmptyState } from "../../components/common/States";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { Tabs } from "../../components/common/Tabs";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import { RecentEvents } from "../dashboard/RecentEvents";
import { OnuTable } from "../onus/OnuTable";
import type { Olt, Pon } from "../../types/domain";
import { formatUptime } from "../../utils/format";

const TABS = [
  { id: "overview", label: "Overview" },
  { id: "pons", label: "PON ports" },
  { id: "onus", label: "ONU inventory" },
  { id: "events", label: "Events" },
];

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="fact">
      <dt className="fact__label">{label}</dt>
      <dd className="fact__value">{children}</dd>
    </div>
  );
}

function ponColumns(): Column<Pon>[] {
  return [
    { key: "name", header: "PON port", render: (pon) => <span className="mono">{pon.name}</span> },
    { key: "total", header: "ONUs", align: "right", render: (pon) => pon.counts.total },
    { key: "online", header: "Online", align: "right", render: (pon) => pon.counts.online },
    { key: "offline", header: "Offline", align: "right", render: (pon) => pon.counts.offline },
    { key: "unknown", header: "Unknown", align: "right", render: (pon) => pon.counts.unknown },
    {
      key: "lowOptical",
      header: "Low RX",
      align: "right",
      render: (pon) => (pon.lowOpticalCount > 0 ? <span className="cell-warning">{pon.lowOpticalCount}</span> : 0),
    },
    { key: "gasp", header: "Gasp", align: "right", render: (pon) => pon.dyingGaspCount24h },
    { key: "flap", header: "Flap", align: "right", render: (pon) => pon.flappingCount },
    {
      key: "collection",
      header: "Collection",
      render: (pon) => <CollectionBadge status={pon.collectionStatus} />,
    },
    {
      key: "lastCollection",
      header: "Last collection",
      render: (pon) => <FreshnessIndicator timestamps={pon.lastCollection} />,
    },
  ];
}

export function OltDetailPage() {
  const { oltId } = useParams<{ oltId: string }>();
  const data = MOCK_OVERVIEW;
  const [activeTab, setActiveTab] = useState("overview");

  const olt: Olt | undefined = data.olts.find((candidate) => candidate.id === oltId);

  if (olt === undefined) {
    return (
      <>
        <PageHeader title="OLT not found" subtitle={`No device with id ${oltId ?? "(none)"}`} />
        <EmptyState
          title="Unknown OLT"
          description="The link may be stale. Return to the inventory and pick a device."
        />
        <p style={{ marginTop: "var(--space-4)" }}>
          <Link className="btn btn--small" to="/olts">
            ← OLT Inventory
          </Link>
        </p>
      </>
    );
  }

  const pons = data.pons.filter((pon) => pon.oltId === olt.id);
  const onus = data.onus.filter((onu) => onu.oltId === olt.id);
  const events = data.events.filter((event) => event.oltId === olt.id);
  const online = onus.filter((onu) => onu.status === "online").length;
  const offline = onus.filter((onu) => onu.status === "offline").length;
  const unknown = onus.filter((onu) => onu.status === "unknown").length;

  return (
    <>
      <PageHeader
        title={olt.name}
        subtitle={`${olt.vendor} ${olt.model} · ${olt.technology} · ${olt.managementAddress} · ${olt.connection.toUpperCase()}`}
        isMock
        actions={
          <Link className="btn btn--small" to="/olts">
            ← OLT Inventory
          </Link>
        }
      />

      <dl className="facts">
        <Fact label="Reachability">
          {olt.reachable ? (
            <span className="badge badge--online">Reachable</span>
          ) : (
            <span className="badge badge--offline">Unreachable</span>
          )}
        </Fact>
        <Fact label="Hardware">{olt.hardwareVersion}</Fact>
        <Fact label="Software">
          <span className="mono">{olt.firmware}</span>
        </Fact>
        <Fact label="Uptime">{formatUptime(olt.uptimeSeconds)}</Fact>
        <Fact label="Device clock">
          {olt.lastCollection?.quality === "trusted" ? (
            "trusted"
          ) : (
            <span className="cell-warning">untrusted — device time is suspect</span>
          )}
        </Fact>
        <Fact label="Last collection">
          <FreshnessIndicator timestamps={olt.lastCollection} />
        </Fact>
        <Fact label="Collection">
          <CollectionBadge status={olt.collectionStatus} />
        </Fact>
      </dl>

      <Tabs items={TABS} activeId={activeTab} onChange={setActiveTab} label="OLT sections" />

      {activeTab === "overview" ? (
        <>
          <section className="kpi-grid" aria-label="Device summary">
            <MetricCard label="PON ports" value={pons.length} />
            <MetricCard label="Total ONUs" value={onus.length} />
            <MetricCard label="Online" value={online} tone="online" />
            <MetricCard label="Offline" value={offline} tone="offline" />
            <MetricCard label="Unknown" value={unknown} tone="unknown" hint="not offline" />
          </section>
          <section className="card" aria-label="Recent events">
            <div className="card__header">
              <h2 className="card__title">Recent events</h2>
              <span className="card__aside">this device only</span>
            </div>
            <div className="card__body card__body--flush">
              <RecentEvents events={events.slice(0, 5)} olts={[olt]} />
            </div>
          </section>
        </>
      ) : null}

      {activeTab === "pons" ? (
        <section className="card" aria-label="PON ports">
          <div className="card__body card__body--flush">
            <DataTable
              columns={ponColumns()}
              rows={pons}
              getRowKey={(pon) => pon.id}
              caption="PON ports"
              emptyMessage="This device exposes no PON port."
            />
          </div>
        </section>
      ) : null}

      {activeTab === "onus" ? (
        <section className="card" aria-label="ONU inventory">
          <div className="card__body card__body--flush">
            <OnuTable onus={onus} olt={olt} emptyMessage="No ONU collected for this device." />
          </div>
        </section>
      ) : null}

      {activeTab === "events" ? (
        <section className="card" aria-label="Events">
          <div className="card__body card__body--flush">
            <RecentEvents events={events} olts={[olt]} />
          </div>
        </section>
      ) : null}

      <p className="page-footnote">
        Performance, collection history, configuration and raw evidence tabs arrive with
        stages 4–7. Nothing on this page is fetched — it is the stage-1 mock fixture.
      </p>
    </>
  );
}
