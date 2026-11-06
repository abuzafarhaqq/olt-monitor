import { useState } from "react";
import { Link } from "react-router-dom";
import { DistributionChart } from "../../components/charts/DistributionChart";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FilterBar, Select, TextInput } from "../../components/common/FilterBar";
import { MetricCard } from "../../components/common/MetricCard";
import { OpticalBadge } from "../../components/common/OpticalBadge";
import { PageHeader } from "../../components/common/PageHeader";
import { StatusBadge } from "../../components/common/StatusBadge";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import type { Onu } from "../../types/domain";
import { formatDbm } from "../../utils/format";
import { getOltMap, severityOf } from "../onus/selectors";
import {
  EMPTY_OPTICAL_FILTERS,
  getHealthByPon,
  getOpticalKpis,
  getOpticalOnus,
  getRxDistribution,
  type OpticalFilters,
  type PonOpticalHealth,
} from "./selectors";

export function OpticalDiagnosticsPage() {
  const data = MOCK_OVERVIEW;
  const olts = getOltMap(data);
  const [filters, setFilters] = useState<OpticalFilters>(EMPTY_OPTICAL_FILTERS);

  const filtered = getOpticalOnus(data, filters);
  const kpis = getOpticalKpis(data);
  const buckets = getRxDistribution(filtered, data);
  const health = getHealthByPon(data, filtered);
  const ponOptions = data.pons.filter(
    (pon) => filters.oltId === "" || pon.oltId === filters.oltId,
  );

  const healthColumns: Column<PonOpticalHealth>[] = [
    {
      key: "pon",
      header: "PON",
      render: (row) => (
        <div>
          <span className="mono">{row.pon.name}</span>
          <div className="cell-sub">{row.oltName}</div>
        </div>
      ),
    },
    { key: "total", header: "ONUs", align: "right", render: (row) => row.total },
    { key: "withReading", header: "With reading", align: "right", render: (row) => row.withReading },
    {
      key: "low",
      header: "Low RX",
      align: "right",
      render: (row) => (row.low > 0 ? <span className="cell-warning">{row.low}</span> : 0),
    },
    {
      key: "critical",
      header: "Critical",
      align: "right",
      render: (row) => (row.critical > 0 ? <span className="cell-warning">{row.critical}</span> : 0),
    },
    { key: "missing", header: "No reading", align: "right", render: (row) => row.missing },
  ];

  const onuColumns: Column<Onu>[] = [
    {
      key: "onu",
      header: "ONU",
      render: (onu) => (
        <div>
          <Link className="link-strong" to={`/onus/${onu.id}`}>
            ONU {onu.onuId}
          </Link>
          <div className="cell-sub mono">{onu.mac}</div>
        </div>
      ),
    },
    {
      key: "location",
      header: "OLT / PON",
      render: (onu) => (
        <div>
          {olts.get(onu.oltId)?.name ?? onu.oltId}
          <div className="cell-sub mono">
            {data.pons.find((pon) => pon.id === onu.ponId)?.name ?? onu.ponId}
          </div>
        </div>
      ),
    },
    { key: "state", header: "Status", render: (onu) => <StatusBadge status={onu.status} /> },
    { key: "rx", header: "RX", align: "right", render: (onu) => formatDbm(onu.rxPowerDbm) },
    {
      key: "verdict",
      header: "Verdict",
      render: (onu) => <OpticalBadge severity={severityOf(onu, olts)} />,
    },
  ];

  return (
    <>
      <PageHeader
        title="Optical Diagnostics"
        subtitle="Readings, thresholds and gaps — missing data is not a fault."
        isMock
      />

      <section className="kpi-grid" aria-label="Optical metrics">
        <MetricCard label="With reading" value={kpis.withReading} hint="RX reported" />
        <MetricCard label="Low" value={kpis.low} tone="warning" hint="below warning threshold" />
        <MetricCard label="Critical" value={kpis.critical} tone="offline" />
        <MetricCard label="No reading" value={kpis.missing} tone="unknown" hint="command ran, no value" />
        <MetricCard label="Unsupported" value={kpis.unsupported} tone="unknown" hint="no optical command" />
        <MetricCard label="No threshold" value={kpis.unconfigured} tone="unknown" hint="pair not calibrated" />
      </section>

      <div className="callout" role="note">
        <strong>A missing reading is not an offline ONU.</strong> {kpis.missing} ONUs have no
        optical sample and {kpis.unsupported} do not expose optics at all. Neither says anything
        about the device being down — only collection can say that.
      </div>

      <div className="dashboard-grid">
        <div className="dashboard-column">
          <section className="card" aria-label="RX distribution">
            <div className="card__header">
              <h2 className="card__title">RX distribution</h2>
              <span className="card__aside">2 dBm buckets · colour = dominant verdict</span>
            </div>
            <div className="card__body">
              <DistributionChart buckets={buckets} />
            </div>
          </section>

          <section className="card" aria-label="Optical health by PON">
            <div className="card__header">
              <h2 className="card__title">Optical health by PON</h2>
              <span className="card__aside">respects the filters</span>
            </div>
            <div className="card__body card__body--flush">
              <DataTable
                columns={healthColumns}
                rows={health}
                getRowKey={(row) => row.pon.id}
                caption="Optical health by PON"
                emptyMessage="No PON matches these filters."
              />
            </div>
          </section>
        </div>

        <div className="dashboard-column">
          <section className="card" aria-label="Optical filters">
            <div className="card__body">
              <FilterBar
                summary={`${filtered.length} of ${data.onus.length}`}
                onReset={() => setFilters(EMPTY_OPTICAL_FILTERS)}
              >
                <Select
                  id="optical-olt"
                  label="OLT"
                  value={filters.oltId}
                  onChange={(oltId) => setFilters((current) => ({ ...current, oltId, ponId: "" }))}
                  options={[
                    { value: "", label: "All OLTs" },
                    ...data.olts.map((olt) => ({ value: olt.id, label: olt.name })),
                  ]}
                />
                <Select
                  id="optical-pon"
                  label="PON"
                  value={filters.ponId}
                  onChange={(ponId) => setFilters((current) => ({ ...current, ponId }))}
                  options={[
                    { value: "", label: "All PONs" },
                    ...ponOptions.map((pon) => ({ value: pon.id, label: pon.name })),
                  ]}
                />
                <Select
                  id="optical-vendor"
                  label="Vendor"
                  value={filters.vendor}
                  onChange={(vendor) => setFilters((current) => ({ ...current, vendor }))}
                  options={[
                    { value: "", label: "All" },
                    { value: "VSOL", label: "VSOL" },
                    { value: "C-Data", label: "C-Data" },
                  ]}
                />
                <Select
                  id="optical-severity"
                  label="Verdict"
                  value={filters.severity}
                  onChange={(severity) => setFilters((current) => ({ ...current, severity }))}
                  options={[
                    { value: "", label: "All" },
                    { value: "ok", label: "OK" },
                    { value: "warning", label: "Low" },
                    { value: "critical", label: "Critical" },
                    { value: "missing", label: "No reading" },
                    { value: "unsupported", label: "Unsupported" },
                    { value: "unconfigured", label: "No threshold" },
                  ]}
                />
                <TextInput
                  id="optical-min"
                  label="RX min (dBm)"
                  value={filters.minDbm}
                  placeholder="-35"
                  onChange={(minDbm) => setFilters((current) => ({ ...current, minDbm }))}
                />
                <TextInput
                  id="optical-max"
                  label="RX max (dBm)"
                  value={filters.maxDbm}
                  placeholder="-17"
                  onChange={(maxDbm) => setFilters((current) => ({ ...current, maxDbm }))}
                />
              </FilterBar>
            </div>
          </section>

          <section className="card" aria-label="ONUs with readings">
            <div className="card__body card__body--flush">
              <DataTable
                columns={onuColumns}
                rows={filtered}
                getRowKey={(onu) => onu.id}
                caption="ONUs with optical readings"
                emptyMessage="No ONU matches these filters."
              />
            </div>
          </section>
        </div>
      </div>
    </>
  );
}
