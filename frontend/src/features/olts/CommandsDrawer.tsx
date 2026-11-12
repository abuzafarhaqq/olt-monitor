import { useEffect, useState } from "react";
import { DataTable, type Column } from "../../components/common/DataTable";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import { EmptyState, ErrorState, LoadingState } from "../../components/common/States";
import { listCommands, type DiscoveredCommand } from "../../services/oltApi";

interface CommandsDrawerProps {
  oltId: string;
  oltName: string;
  onClose: () => void;
}

const KIND_TONE: Record<DiscoveredCommand["kind"], string> = {
  read: "badge--online",
  action: "badge--offline",
  param: "badge--unknown",
  unknown: "badge--warning",
};

const KIND_LABEL: Record<DiscoveredCommand["kind"], string> = {
  read: "read",
  action: "never run",
  param: "needs arg",
  unknown: "unclassified",
};

/**
 * What the last crawl found on this device.
 *
 * Read commands are the ones the platform may execute. "never run" marks verbs
 * the device offers that change something — they are listed so an operator can
 * see the device's real surface, and nothing more.
 */
export function CommandsDrawer({ oltId, oltName, onClose }: CommandsDrawerProps) {
  const [commands, setCommands] = useState<DiscoveredCommand[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    listCommands(oltId)
      .then((rows) => {
        if (alive) setCommands(rows);
      })
      .catch((caught: unknown) => {
        if (alive) setError(caught instanceof Error ? caught.message : "Could not load commands");
      });
    return () => {
      alive = false;
    };
  }, [oltId]);

  const columns: Column<DiscoveredCommand>[] = [
    {
      key: "path",
      header: "Command",
      render: (command) => <span className="mono">{command.path}</span>,
    },
    {
      key: "kind",
      header: "Kind",
      render: (command) => (
        <span className={`badge ${KIND_TONE[command.kind]}`}>{KIND_LABEL[command.kind]}</span>
      ),
    },
    {
      key: "help",
      header: "Device says",
      render: (command) => <span className="cell-sub">{command.help}</span>,
    },
  ];

  const counts = commands?.reduce<Record<string, number>>((acc, command) => {
    acc[command.kind] = (acc[command.kind] ?? 0) + 1;
    return acc;
  }, {});

  return (
    <DetailDrawer
      size="wide"
      title={`Discovered commands — ${oltName}`}
      subtitle={
        commands === null
          ? "loading"
          : `${commands.length} commands · ${counts?.read ?? 0} readable · ${counts?.action ?? 0} never run`
      }
      onClose={onClose}
      footer={
        <button type="button" className="btn" onClick={onClose}>
          Close
        </button>
      }
    >
      {error === null ? null : <ErrorState description={error} />}
      {commands === null && error === null ? <LoadingState label="Loading commands…" /> : null}
      {commands !== null && commands.length === 0 ? (
        <EmptyState
          title="No crawl yet"
          description="Press Crawl on this device. The crawler walks the CLI with '?' and stores what it finds."
        />
      ) : null}
      {commands !== null && commands.length > 0 ? (
        <div className="table-scroll">
          <DataTable
            columns={columns}
            rows={commands}
            getRowKey={(command) => command.path}
            caption="Discovered commands"
            emptyMessage="No command discovered."
          />
        </div>
      ) : null}
    </DetailDrawer>
  );
}
