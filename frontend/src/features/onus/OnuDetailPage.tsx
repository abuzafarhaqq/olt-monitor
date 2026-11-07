import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { OpticalTrendChart } from "../../components/charts/OpticalTrendChart";
import { CollectionBadge } from "../../components/common/CollectionBadge";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import { MetricCard } from "../../components/common/MetricCard";
import { OpticalBadge } from "../../components/common/OpticalBadge";
import { PageHeader } from "../../components/common/PageHeader";
import { RawOutputViewer } from "../../components/common/RawOutputViewer";
import { EmptyState } from "../../components/common/States";
import { StatusBadge } from "../../components/common/StatusBadge";
import { Tabs } from "../../components/common/Tabs";
import { classifyRx, findThreshold } from "../../constants/opticalThresholds";
import { ONU_DIAGNOSTICS } from "../../mock/onuDiagnostics";
import { ErrorState, LoadingState } from "../../components/common/States";
import { useOverview } from "../../hooks/useResource";
import { USE_MOCK_API } from "../../services/client";
import type { OpticalHistoryPoint } from "../../types/domain";
import { formatDateTime, formatDistance, formatDbm } from "../../utils/format";
import { RecentEvents } from "../dashboard/RecentEvents";

const TABS = [
  { id: "overview", label: "Overview" },
  { id: "optical", label: "Optical" },
  { id: "history", label: "History" },
  { id: "events", label: "Events" },
  { id: "incidents", label: "Incidents" },
  { id: "raw", label: "Raw evidence" },
];

export function OnuDetailPage() {
  const { onuId } = useParams<{ onuId: string }>();
  const [activeTab, setActiveTab] = useState("overview");
  const { data, loading, error } = useOverview();

  if (loading) return <LoadingState label="Loading ONU…" />;
  if (data === null) return <ErrorState description={error ?? "No ONU data."} />;

  const onu = data.onus.find((candidate) => candidate.id === onuId);
  const olt = data.olts.find((candidate) => candidate.id === onu?.oltId);
  const pon = data.pons.find((candidate) => candidate.id === onu?.ponId);

  if (onu === undefined || olt === undefined) {
    return (
      <>
        <PageHeader title="ONU not found" subtitle={`No ONU with id ${onuId ?? "(none)"}`} />
        <EmptyState title="Unknown ONU" description="The link may be stale. Open the PON Explorer and pick an ONU." />
        <p style={{ marginTop: "var(--space-4)" }}>
          <Link className="btn btn--small" to="/onus">
            ← ONU Management
          </Link>
        </p>
      </>
    );
  }

  const diagnostics = ONU_DIAGNOSTICS.get(onu.id);
  const threshold = findThreshold(onu.vendor, olt.technology);
  const severity = classifyRx(onu.rxPowerDbm, onu.vendor, olt.technology, onu.opticalSupported);
  const measurements = diagnostics?.measurements;

  const events = data.events.filter(
    (event) => event.oltId === onu.oltId && event.onuId === onu.onuId,
  );
  const incidents = data.incidents.filter(
    (incident) =>
      incident.oltId === onu.oltId && (incident.ponId === null || incident.ponId === onu.ponId),
  );

  const rxTone =
    severity === "critical" ? "offline" : severity === "warning" ? "warning" : severity === "ok" ? "online" : "unknown";

  const historyColumns: Column<OpticalHistoryPoint>[] = [
    { key: "at", header: "Sample", render: (point) => <span className="tabular">{formatDateTime(point.at)}</span> },
    { key: "rx", header: "RX", align: "right", render: (point) => formatDbm(point.rxPowerDbm) },
    {
      key: "severity",
      header: "Verdict",
      render: (point) => (
        <OpticalBadge
          severity={classifyRx(point.rxPowerDbm, onu.vendor, olt.technology, onu.opticalSupported)}
        />
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={`ONU ${onu.onuId} · ${onu.mac}`}
        subtitle={`${olt.name} · ${pon?.name ?? onu.ponId} · ${onu.vendor} ${onu.model}`}
        isMock={USE_MOCK_API}
        actions={
          <Link className="btn btn--small" to="/onus">
            ← ONU Management
          </Link>
        }
      />

      <dl className="facts">
        <div className="fact">
          <dt className="fact__label">Status</dt>
          <dd className="fact__value">
            <StatusBadge status={onu.status} />
          </dd>
        </div>
        <div className="fact">
          <dt className="fact__label">Collection</dt>
          <dd className="fact__value">
            <CollectionBadge status={onu.collectionStatus} />
          </dd>
        </div>
        <div className="fact">
          <dt className="fact__label">Last online</dt>
          <dd className="fact__value">
            <FreshnessIndicator timestamps={onu.lastOnlineAt} />
          </dd>
        </div>
        <div className="fact">
          <dt className="fact__label">Last registration</dt>
          <dd className="fact__value">
            <FreshnessIndicator timestamps={onu.lastRegistrationAt} />
          </dd>
        </div>
        <div className="fact">
          <dt className="fact__label">Last deregistration</dt>
          <dd className="fact__value">
            {onu.lastDeregistrationAt === null ? (
              "none recorded"
            ) : (
              <FreshnessIndicator timestamps={onu.lastDeregistrationAt} />
            )}
          </dd>
        </div>
        <div className="fact">
          <dt className="fact__label">Last measurement</dt>
          <dd className="fact__value">
            {measurements?.measuredAt === null || measurements === undefined
              ? "none"
              : formatDateTime(measurements.measuredAt)}
          </dd>
        </div>
      </dl>

      <Tabs items={TABS} activeId={activeTab} onChange={setActiveTab} label="ONU sections" />

      {activeTab === "overview" ? (
        <>
          <div className="kpi-grid">
            <MetricCard
              label="RX power"
              value={formatDbm(onu.rxPowerDbm)}
              tone={rxTone}
              hint={
                threshold === null
                  ? "no threshold configured"
                  : `warn ${threshold.warningDbm} · crit ${threshold.criticalDbm} dBm`
              }
            />
            {measurements?.txPowerDbm.supported === true ? (
              <MetricCard
                label="TX power"
                value={measurements.txPowerDbm.value === null ? "—" : `${measurements.txPowerDbm.value.toFixed(2)} dBm`}
              />
            ) : null}
            {measurements?.temperatureC.supported === true ? (
              <MetricCard
                label="Temperature"
                value={measurements.temperatureC.value === null ? "—" : `${measurements.temperatureC.value.toFixed(1)} °C`}
              />
            ) : null}
            {measurements?.supplyVoltageV.supported === true ? (
              <MetricCard
                label="Supply voltage"
                value={measurements.supplyVoltageV.value === null ? "—" : `${measurements.supplyVoltageV.value.toFixed(2)} V`}
              />
            ) : null}
            {measurements?.txBiasCurrentMa.supported === true ? (
              <MetricCard
                label="TX bias"
                value={measurements.txBiasCurrentMa.value === null ? "—" : `${measurements.txBiasCurrentMa.value.toFixed(1)} mA`}
              />
            ) : null}
            <MetricCard label="Distance" value={formatDistance(onu.distanceMeters)} />
            <MetricCard
              label="Optical verdict"
              value={<OpticalBadge severity={severity} />}
              tone={rxTone}
            />
          </div>

          <p className="page-footnote">
            Metrics the adapter does not expose for this model are omitted, not shown as zero.
            {measurements !== undefined &&
            (!measurements.txPowerDbm.supported || !measurements.temperatureC.supported)
              ? " Not exposed here: temperature, voltage or TX bias."
              : ""}
          </p>
        </>
      ) : null}

      {activeTab === "optical" ? (
        <section className="card" aria-label="Optical trend">
          <div className="card__header">
            <h2 className="card__title">RX power · last 24 hours</h2>
            <span className="card__aside">
              {threshold === null
                ? "no threshold for this vendor/model"
                : `${onu.vendor}/${olt.technology} thresholds`}
            </span>
          </div>
          <div className="card__body">
            {threshold === null ? (
              <p className="cell-sub">
                This vendor/model pair has no calibrated threshold, so no verdict and no
                threshold bands are drawn. The raw samples remain available in History.
              </p>
            ) : diagnostics === undefined ? (
              <p className="cell-sub">No optical history stored.</p>
            ) : (
              <OpticalTrendChart
                points={diagnostics.history}
                warningDbm={threshold.warningDbm}
                criticalDbm={threshold.criticalDbm}
              />
            )}
          </div>
        </section>
      ) : null}

      {activeTab === "history" ? (
        <section className="card" aria-label="Optical history">
          <div className="card__body card__body--flush">
            <DataTable
              columns={historyColumns}
              rows={diagnostics?.history ?? []}
              getRowKey={(point) => point.at}
              caption="Optical history"
              emptyMessage="No samples stored."
            />
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

      {activeTab === "incidents" ? (
        <section className="card" aria-label="Incidents">
          <div className="card__body">
            {incidents.length === 0 ? (
              <p className="cell-sub">No incident is associated with this ONU or its PON.</p>
            ) : (
              <ul className="incident-list">
                {incidents.map((incident) => (
                  <li className="incident-row" key={incident.id}>
                    <span className="mono">{incident.id}</span>
                    <span>{incident.title}</span>
                    <span className="badge badge--investigating">{incident.state}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      ) : null}

      {activeTab === "raw" ? (
        <section className="card" aria-label="Raw evidence">
          <div className="card__body">
            <RawOutputViewer evidence={diagnostics?.evidence ?? []} />
          </div>
        </section>
      ) : null}
    </>
  );
}
