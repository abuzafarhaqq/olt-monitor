import { useState } from "react";
import { Link } from "react-router-dom";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FilterBar, Select, TextInput } from "../../components/common/FilterBar";
import { MetricCard } from "../../components/common/MetricCard";
import { OpticalBadge } from "../../components/common/OpticalBadge";
import { PageHeader } from "../../components/common/PageHeader";
import { StatusBadge } from "../../components/common/StatusBadge";
import { ErrorState, LoadingState } from "../../components/common/States";
import { useOverview } from "../../hooks/useResource";
import { USE_MOCK_API } from "../../services/client";
import type { Onu } from "../../types/domain";
import { formatDistance, formatDbm, formatRelative } from "../../utils/format";
import {
  EMPTY_ONU_LIST_FILTERS,
  filterOnus,
  getOltMap,
  severityOf,
  type OnuListFilters,
} from "./selectors";

interface VendorGroup {
  vendor: string;
  model: string;
  total: number;
  online: number;
  unprovisioned: number;
}

/** One PON carries many vendors' ONUs, so the estate is summarised by model. */
function groupByVendor(onus: Onu[]): VendorGroup[] {
  const groups = new Map<string, VendorGroup>();
  for (const onu of onus) {
    const vendor = onu.vendorId ?? "unknown";
    const model = onu.model ?? "unknown";
    const key = `${vendor}/${model}`;
    const group = groups.get(key) ?? { vendor, model, total: 0, online: 0, unprovisioned: 0 };
    group.total += 1;
    if (onu.status === "online" && onu.provisioned !== false) group.online += 1;
    if (onu.provisioned === false) group.unprovisioned += 1;
    groups.set(key, group);
  }
  return [...groups.values()].sort((a, b) => b.total - a.total);
}

function VendorSummary({ onus }: { onus: Onu[] }) {
  const groups = groupByVendor(onus);
  const columns: Column<VendorGroup>[] = [
    { key: "vendor", header: "Vendor", render: (g) => <span className="mono">{g.vendor}</span> },
    { key: "model", header: "Model", render: (g) => g.model },
    { key: "total", header: "ONUs", align: "right", render: (g) => g.total },
    { key: "online", header: "Online", align: "right", render: (g) => g.online },
    {
      key: "unprovisioned",
      header: "Unprovisioned",
      align: "right",
      render: (g) =>
        g.unprovisioned > 0 ? <span className="cell-warning">{g.unprovisioned}</span> : 0,
    },
  ];

  return (
    <section className="card" aria-label="ONUs by vendor" style={{ marginBottom: "var(--space-4)" }}>
      <div className="card__header">
        <h2 className="card__title">ONUs by vendor and model</h2>
        <span className="card__aside">{groups.length} combinations across the estate</span>
      </div>
      <div className="card__body card__body--flush">
        <DataTable
          columns={columns}
          rows={groups}
          getRowKey={(g) => `${g.vendor}/${g.model}`}
          caption="ONUs by vendor"
          emptyMessage="No ONU has been collected yet."
        />
      </div>
    </section>
  );
}

export function OnuListPage() {
  const { data, loading, error } = useOverview();
  const [filters, setFilters] = useState<OnuListFilters>(EMPTY_ONU_LIST_FILTERS);

  if (loading) return <LoadingState label="Loading ONUs…" />;
  if (data === null) return <ErrorState description={error ?? "No ONU data."} />;

  const olts = getOltMap(data);
  const visible = filterOnus(data, filters);
  const ponOptions = data.pons.filter(
    (pon) => filters.oltId === "" || pon.oltId === filters.oltId,
  );

  const columns: Column<Onu>[] = [
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
    {
      key: "vendor",
      header: "Vendor / model",
      render: (onu) => (
        <div>
          {onu.vendor}
          <div className="cell-sub">{onu.model}</div>
        </div>
      ),
    },
    { key: "status", header: "Status", render: (onu) => <StatusBadge status={onu.status} /> },
    {
      key: "optical",
      header: "Optical",
      render: (onu) => <OpticalBadge severity={severityOf(onu, olts)} />,
    },
    { key: "rx", header: "RX", align: "right", render: (onu) => formatDbm(onu.rxPowerDbm) },
    {
      key: "distance",
      header: "Distance",
      align: "right",
      render: (onu) => formatDistance(onu.distanceMeters),
    },
    {
      key: "dereg",
      header: "Dereg 24h",
      align: "right",
      render: (onu) =>
        onu.deregistrations24h >= 3 ? (
          <span className="cell-warning">{onu.deregistrations24h}</span>
        ) : (
          onu.deregistrations24h
        ),
    },
    {
      key: "lastOnline",
      header: "Last online",
      render: (onu) =>
        onu.lastOnlineAt === null ? (
          <span className="cell-sub">never</span>
        ) : (
          <span className="tabular">{formatRelative(onu.lastOnlineAt.ingestedAt)}</span>
        ),
    },
  ];

  return (
    <>
      <PageHeader
        title="ONU Management"
        subtitle={`${data.onus.length} ONUs across ${data.olts.length} OLTs`}
        isMock={USE_MOCK_API}
      />

      <section className="kpi-grid" aria-label="ONU metrics">
        <MetricCard label="Total" value={data.onus.length} />
        <MetricCard
          label="Online"
          value={data.onus.filter((onu) => onu.status === "online").length}
          tone="online"
        />
        <MetricCard
          label="Offline"
          value={data.onus.filter((onu) => onu.status === "offline").length}
          tone="offline"
        />
        <MetricCard
          label="Unknown"
          value={data.onus.filter((onu) => onu.status === "unknown").length}
          tone="unknown"
          hint="not offline"
        />
        <MetricCard
          label="Showing"
          value={visible.length}
          hint="after filters"
          tone="info"
        />
      </section>

      <VendorSummary onus={data.onus} />

      <section className="card" aria-label="ONUs">
        <div className="card__body">
          <FilterBar
            summary={`${visible.length} of ${data.onus.length} ONUs`}
            onReset={() => setFilters(EMPTY_ONU_LIST_FILTERS)}
          >
            <TextInput
              id="onu-list-search"
              label="ONU ID or MAC"
              value={filters.query}
              placeholder="e.g. 7 or 02:1A:2B"
              onChange={(query) => setFilters((current) => ({ ...current, query }))}
            />
            <Select
              id="onu-list-olt"
              label="OLT"
              value={filters.oltId}
              onChange={(oltId) => setFilters((current) => ({ ...current, oltId, ponId: "" }))}
              options={[
                { value: "", label: "All OLTs" },
                ...data.olts.map((olt) => ({ value: olt.id, label: olt.name })),
              ]}
            />
            <Select
              id="onu-list-pon"
              label="PON"
              value={filters.ponId}
              onChange={(ponId) => setFilters((current) => ({ ...current, ponId }))}
              options={[
                { value: "", label: "All PONs" },
                ...ponOptions.map((pon) => ({ value: pon.id, label: pon.name })),
              ]}
            />
            <Select
              id="onu-list-state"
              label="State"
              value={filters.state}
              onChange={(state) => setFilters((current) => ({ ...current, state }))}
              options={[
                { value: "", label: "All" },
                { value: "online", label: "Online" },
                { value: "offline", label: "Offline" },
                { value: "unknown", label: "Unknown" },
              ]}
            />
            <Select
              id="onu-list-optical"
              label="Optical"
              value={filters.optical}
              onChange={(optical) => setFilters((current) => ({ ...current, optical }))}
              options={[
                { value: "", label: "All" },
                { value: "ok", label: "OK" },
                { value: "warning", label: "Low" },
                { value: "critical", label: "Critical" },
                { value: "missing", label: "No reading" },
                { value: "unsupported", label: "Unsupported" },
              ]}
            />
          </FilterBar>
        </div>
        <div className="card__body card__body--flush">
          <DataTable
            columns={columns}
            rows={visible}
            getRowKey={(onu) => onu.id}
            caption="ONUs"
            emptyMessage="No ONU matches these filters."
          />
        </div>
      </section>
    </>
  );
}
