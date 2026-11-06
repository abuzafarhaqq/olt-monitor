import type { EventSeverity, NetworkEvent, Olt } from "../../types/domain";
import { formatRelative } from "../../utils/format";

interface RecentEventsProps {
  events: NetworkEvent[];
  olts: Olt[];
}

const SEVERITY_TONE: Record<EventSeverity, string> = {
  info: "badge--info",
  warning: "badge--warning",
  critical: "badge--offline",
};

export function RecentEvents({ events, olts }: RecentEventsProps) {
  const oltNames = new Map(olts.map((olt) => [olt.id, olt.name]));

  if (events.length === 0) {
    return <p className="card__body">No events recorded.</p>;
  }

  return (
    <div className="event-list">
      {events.map((event) => {
        const ponLabel = event.ponId ? event.ponId.split("-").pop() : null;
        const source = [
          oltNames.get(event.oltId) ?? event.oltId,
          ponLabel,
          event.onuId !== null ? `ONU ${event.onuId}` : null,
        ]
          .filter(Boolean)
          .join(" · ");

        return (
          <div className="event-row" key={event.id}>
            <span className="event-row__time">{formatRelative(event.timestamps.ingestedAt)}</span>
            <div>
              <div className="event-row__message">{event.message}</div>
              <div className="event-row__source">{source}</div>
            </div>
            <span className={`badge ${SEVERITY_TONE[event.severity]}`}>{event.severity}</span>
          </div>
        );
      })}
    </div>
  );
}
