import { useState } from "react";
import { Link } from "react-router-dom";
import { CollectionBadge } from "../../components/common/CollectionBadge";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FilterBar, Select, TextInput } from "../../components/common/FilterBar";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import { PageHeader } from "../../components/common/PageHeader";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import { formatCount } from "../../utils/format";
import {
  EMPTY_OLT_FILTERS,
  filterOltRows,
  getOltRows,
  type OltFilters,
  type OltRow,
} from "./selectors";

export function OltInventoryPage() {
  const data = MOCK_OVERVIEW;
  const rows = getOltRows(data);
  const [filters, setFilters] = useState<OltFilters>(EMPTY_OLT_FILTERS);

  const visible = filterOltRows(rows, filters);

  const columns: Column<OltRow>[] = [
    {
      key: "name",
      header: "OLT",
      render: ({ olt }) => (
        <div>
          <Link className="link-strong" to={`/olts/${olt.id}`}>
            {olt.name}
          </Link>
          <div className="cell-sub">
            {olt.vendor} {olt.model}
          </div>
        </div>
      ),
    },
    { key: "technology", header: "PON", render: ({ olt }) => olt.technology },
    {
      key: "management",
      header: "Management",
      render: ({ olt }) => (
        <div>
          <span className="mono">{olt.managementAddress}</span>
          <div className="cell-sub">{olt.connection.toUpperCase()}</div>
        </div>
      ),
    },
    {
      key: "reachability",
      header: "Reachability",
      render: ({ olt }) =>
        olt.reachable ? (
          <span className="badge badge--online">Reachable</span>
        ) : (
          <span className="badge badge--offline">Unreachable</span>
        ),
    },
    {
      key: "firmware",
      header: "Firmware",
      render: ({ olt }) => <span className="mono cell-sub">{olt.firmware}</span>,
    },
    { key: "pons", header: "PONs", align: "right", render: (row) => row.ponCount },
    { key: "onus", header: "ONUs", align: "right", render: (row) => formatCount(row.onuCount) },
    {
      key: "lastCollection",
      header: "Last collection",
      render: ({ olt }) => <FreshnessIndicator timestamps={olt.lastCollection} />,
    },
    {
      key: "collection",
      header: "Collection",
      render: ({ olt }) => <CollectionBadge status={olt.collectionStatus} />,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: ({ olt }) => (
        <div className="row-actions">
          <Link className="btn btn--small" to={`/olts/${olt.id}`}>
            Open
          </Link>
          <Link className="btn btn--small" to={`/olts/${olt.id}/edit`}>
            Edit
          </Link>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="OLT Inventory"
        subtitle={`${rows.length} devices · ${data.olts.filter((olt) => olt.reachable).length} reachable`}
        isMock
        actions={
          <Link className="btn btn--primary" to="/olts/new">
            Add OLT
          </Link>
        }
      />

      <section className="card" aria-label="OLT inventory">
        <div className="card__body">
          <FilterBar
            summary={`${visible.length} of ${rows.length} OLTs`}
            onReset={() => setFilters(EMPTY_OLT_FILTERS)}
          >
            <TextInput
              id="olt-search"
              label="Search"
              value={filters.query}
              placeholder="name, vendor, model or address"
              onChange={(query) => setFilters((current) => ({ ...current, query }))}
            />
            <Select
              id="olt-technology"
              label="Technology"
              value={filters.technology}
              onChange={(technology) => setFilters((current) => ({ ...current, technology }))}
              options={[
                { value: "", label: "All" },
                { value: "EPON", label: "EPON" },
                { value: "GPON", label: "GPON" },
              ]}
            />
            <Select
              id="olt-reachability"
              label="Reachability"
              value={filters.reachability}
              onChange={(reachability) => setFilters((current) => ({ ...current, reachability }))}
              options={[
                { value: "", label: "All" },
                { value: "reachable", label: "Reachable" },
                { value: "unreachable", label: "Unreachable" },
              ]}
            />
          </FilterBar>
        </div>
        <div className="card__body card__body--flush">
          <DataTable
            columns={columns}
            rows={visible}
            getRowKey={(row) => row.olt.id}
            caption="OLT inventory"
            emptyMessage="No OLT matches these filters."
          />
        </div>
      </section>

    </>
  );
}
