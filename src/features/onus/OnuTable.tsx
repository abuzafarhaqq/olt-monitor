import { classifyRx } from "../../constants/opticalThresholds";
import { DataTable, type Column } from "../../components/common/DataTable";
import { OpticalBadge } from "../../components/common/OpticalBadge";
import { StatusBadge } from "../../components/common/StatusBadge";
import type { Onu, Olt } from "../../types/domain";
import { formatDistance, formatDbm } from "../../utils/format";

interface OnuTableProps {
  onus: Onu[];
  /** Needed for model-specific optical thresholds. */
  olt: Olt;
  emptyMessage?: string;
}

/** One ONU table, used by PON Explorer and by the OLT detail page. */
export function OnuTable({ onus, olt, emptyMessage = "No ONU matches these filters." }: OnuTableProps) {
  const columns: Column<Onu>[] = [
    {
      key: "id",
      header: "ONU ID",
      align: "right",
      render: (onu) => <span className="mono">{onu.onuId}</span>,
    },
    { key: "mac", header: "MAC", render: (onu) => <span className="mono">{onu.mac}</span> },
    { key: "status", header: "Status", render: (onu) => <StatusBadge status={onu.status} /> },
    {
      key: "optical",
      header: "Optical",
      render: (onu) => (
        <OpticalBadge
          severity={classifyRx(onu.rxPowerDbm, onu.vendor, olt.technology, onu.opticalSupported)}
        />
      ),
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
      render: (onu) => (onu.deregistrations24h >= 3 ? (
        <span className="cell-warning">{onu.deregistrations24h}</span>
      ) : (
        onu.deregistrations24h
      )),
    },
    {
      key: "event",
      header: "Last event",
      render: (onu) => <span className="cell-sub mono">{onu.lastEvent ?? "—"}</span>,
    },
  ];

  return <DataTable columns={columns} rows={onus} getRowKey={(onu) => onu.id} caption="ONUs" emptyMessage={emptyMessage} />;
}
