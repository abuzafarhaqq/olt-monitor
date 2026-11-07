import { CollectionBadge } from "../../components/common/CollectionBadge";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import type { Onu, Olt } from "../../types/domain";
import { formatCount } from "../../utils/format";

interface OltHealthTableProps {
  olts: Olt[];
  onus: Onu[];
}

export function OltHealthTable({ olts, onus }: OltHealthTableProps) {
  const onuCountByOlt = new Map<string, number>();
  for (const onu of onus) {
    onuCountByOlt.set(onu.oltId, (onuCountByOlt.get(onu.oltId) ?? 0) + 1);
  }

  const columns: Column<Olt>[] = [
    {
      key: "name",
      header: "OLT",
      render: (olt) => (
        <div>
          <div className="data-table__cell-strong">{olt.name}</div>
          <div className="event-row__source">
            {olt.vendor} {olt.model} · {olt.managementAddress}
          </div>
        </div>
      ),
    },
    {
      key: "technology",
      header: "PON",
      render: (olt) => olt.technology,
    },
    {
      key: "reachability",
      header: "Reachability",
      render: (olt) =>
        olt.reachable ? (
          <span className="badge badge--online">Reachable</span>
        ) : (
          <span className="badge badge--offline">Unreachable</span>
        ),
    },
    {
      key: "onus",
      header: "ONUs",
      align: "right",
      render: (olt) => formatCount(onuCountByOlt.get(olt.id) ?? 0),
    },
    {
      key: "lastCollection",
      header: "Last collection",
      render: (olt) => <FreshnessIndicator timestamps={olt.lastCollection} />,
    },
    {
      key: "collection",
      header: "Collection",
      render: (olt) => <CollectionBadge status={olt.collectionStatus} />,
    },
  ];

  return (
    <DataTable
      columns={columns}
      rows={olts}
      getRowKey={(olt) => olt.id}
      caption="OLT health"
      emptyMessage="No OLT is configured yet."
    />
  );
}
