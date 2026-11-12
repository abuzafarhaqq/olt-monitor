import { useState } from "react";
import { CollectionBadge } from "../../components/common/CollectionBadge";
import { FilterBar, Select, TextInput } from "../../components/common/FilterBar";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { EmptyState } from "../../components/common/States";
import { ErrorState, LoadingState } from "../../components/common/States";
import { useOverview } from "../../hooks/useResource";
import { USE_MOCK_API } from "../../services/client";
import { OnuTable } from "../onus/OnuTable";
import {
  EMPTY_ONU_FILTERS,
  filterPonOnus,
  getPonMetrics,
  getTree,
  getVendorModelOptions,
  type OnuFilters,
} from "./selectors";

export function PonExplorerPage() {
  const { data, loading, error, reload } = useOverview();
  // Empty means "not chosen yet": the first PON is picked once data arrives,
  // because at mount there is nothing to pick from.
  const [selectedPonId, setSelectedPonId] = useState<string>("");
  const [filters, setFilters] = useState<OnuFilters>(EMPTY_ONU_FILTERS);

  if (loading) return <LoadingState label="Loading PONs…" />;
  if (data === null) return <ErrorState description={error ?? "No PON data."} />;

  const branches = getTree(data);
  const activePonId = selectedPonId === "" ? (data.pons[0]?.id ?? "") : selectedPonId;
  const pon = data.pons.find((candidate) => candidate.id === activePonId);
  const olt = data.olts.find((candidate) => candidate.id === pon?.oltId);

  if (pon === undefined || olt === undefined) {
    return (
      <>
        <PageHeader title="PON Explorer" subtitle="No PON selected" isMock={USE_MOCK_API} />
        <EmptyState title="Nothing to show" description="No PON port is known to the platform." />
      </>
    );
  }

  const ponOnus = data.onus.filter((onu) => onu.ponId === pon.id);
  const metrics = getPonMetrics(ponOnus, olt);
  const visibleOnus = filterPonOnus(ponOnus, olt, filters);
  const vendorModels = getVendorModelOptions(ponOnus);

  return (
    <>
      <PageHeader
        title="PON Explorer"
        subtitle="OLT → PON port → ONU. Port names come from each adapter."
        isMock={USE_MOCK_API}
        actions={
          <button type="button" className="btn btn--small" onClick={reload} disabled={loading}>
            Reload
          </button>
        }
      />

      <div className="explorer">
        <nav className="tree" aria-label="OLT and PON tree">
          {branches.map(({ olt: branchOlt, pons }) => (
            <div className="tree__group" key={branchOlt.id}>
              <button
                type="button"
                className={
                  branchOlt.id === olt.id ? "tree__olt tree__olt--active" : "tree__olt"
                }
                onClick={() => setSelectedPonId(pons[0]?.id ?? "")}
                aria-expanded={branchOlt.id === olt.id}
              >
                <span className="tree__name">{branchOlt.name}</span>
                <span className="tree__meta">{branchOlt.technology}</span>
              </button>

              {branchOlt.id === olt.id
                ? pons.map((branchPon) => (
                    <button
                      key={branchPon.id}
                      type="button"
                      className={
                        branchPon.id === pon.id ? "tree__pon tree__pon--active" : "tree__pon"
                      }
                      onClick={() => setSelectedPonId(branchPon.id)}
                      aria-current={branchPon.id === pon.id}
                    >
                      <span className="mono">{branchPon.name}</span>
                      <span className="tree__count">{branchPon.counts.total}</span>
                    </button>
                  ))
                : null}
            </div>
          ))}
        </nav>

        <div className="explorer__main">
          <section className="card" aria-label="PON summary">
            <div className="card__header">
              <h2 className="card__title">
                {olt.name} · <span className="mono">{pon.name}</span>
              </h2>
              <span className="card__aside">
                <CollectionBadge status={pon.collectionStatus} />
              </span>
            </div>
            <div className="card__body">
              <div className="kpi-grid kpi-grid--compact">
                <MetricCard label="Total" value={metrics.total} />
                <MetricCard label="Online" value={metrics.online} tone="online" />
                <MetricCard label="Offline" value={metrics.offline} tone="offline" />
                <MetricCard label="Unknown" value={metrics.unknown} tone="unknown" />
                <MetricCard label="Low RX" value={metrics.lowOptical} tone="warning" />
                <MetricCard label="Dying gasp" value={metrics.dyingGasp} tone="warning" />
                <MetricCard label="Flapping" value={metrics.flapping} tone="warning" />
              </div>
              <div className="pon-meta">
                <span>
                  Last successful collection <FreshnessIndicator timestamps={pon.lastCollection} />
                </span>
              </div>
            </div>
          </section>

          <section className="card" aria-label="ONUs on this PON">
            <div className="card__header">
              <h2 className="card__title">ONUs</h2>
              <span className="card__aside">thresholds for {olt.vendor} {olt.technology}</span>
            </div>

            <div className="card__body">
              <FilterBar
                summary={`${visibleOnus.length} of ${ponOnus.length} ONUs`}
                onReset={() => setFilters(EMPTY_ONU_FILTERS)}
              >
                <TextInput
                  id="onu-search"
                  label="ONU ID or MAC"
                  value={filters.query}
                  placeholder="e.g. 12 or 02:1A:2B"
                  onChange={(query) => setFilters((current) => ({ ...current, query }))}
                />
                <Select
                  id="onu-state"
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
                  id="onu-optical"
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
                <Select
                  id="onu-event"
                  label="Last event"
                  value={filters.event}
                  onChange={(event) => setFilters((current) => ({ ...current, event }))}
                  options={[
                    { value: "", label: "All" },
                    { value: "onu-dying-gasp", label: "Dying gasp" },
                    { value: "onu-deregister", label: "Deregistered" },
                    { value: "link-lost", label: "Link lost" },
                    { value: "onu-auth-failure", label: "Auth failure" },
                    { value: "onu-register", label: "Registered" },
                  ]}
                />
                <Select
                  id="onu-disconnects"
                  label="Deregistrations"
                  value={filters.disconnects}
                  onChange={(disconnects) => setFilters((current) => ({ ...current, disconnects }))}
                  options={[
                    { value: "", label: "Any" },
                    { value: "3", label: "3+ in 24h" },
                  ]}
                />
                <Select
                  id="onu-vendor-model"
                  label="Vendor / model"
                  value={filters.vendorModel}
                  onChange={(vendorModel) => setFilters((current) => ({ ...current, vendorModel }))}
                  options={[
                    { value: "", label: "All" },
                    ...vendorModels.map((value) => ({ value, label: value })),
                  ]}
                />
              </FilterBar>
            </div>

            <div className="card__body card__body--flush">
              <OnuTable onus={visibleOnus} olt={olt} />
            </div>
          </section>
        </div>
      </div>
    </>
  );
}
