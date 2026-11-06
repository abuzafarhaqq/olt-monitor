import type { EventSeverity, NetworkEvent, Olt } from "../../types/domain";
import { formatDateTime, formatRelative } from "../../utils/format";

interface EventTimelineProps {
  events: NetworkEvent[];
  olts: Olt[];
  /** Hide the evidence reference when the list is used as a plain feed. */
  showEvidence?: boolean;
}

const DOT_COLOR: Record<EventSeverity, string> = {
  info: "var(--color-info)",
  warning: "var(--color-warning)",
  critical: "var(--color-offline)",
};

/**
 * Events are observations. The timeline shows *two* clocks: when the platform
 * saw the event, and what the device claimed. When the device clock is
 * untrusted the second line says so instead of quietly presenting it as fact.
 */
export function EventTimeline({ events, olts, showEvidence = true }: EventTimelineProps) {
  const oltNames = new Map(olts.map((olt) => [olt.id, olt.name]));

  if (events.length === 0) {
    return <p className="cell-sub">No event matches these filters.</p>;
  }

  return (
    <ol className="timeline">
      {events.map((event) => {
        const ponLabel = event.ponId === null ? null : event.ponId.split("-").pop();
        return (
          <li className="timeline__item" key={event.id}>
            <span
              className="timeline__dot"
              style={{ background: DOT_COLOR[event.severity] }}
              aria-hidden="true"
            />
            <div className="timeline__body">
              <div className="timeline__head">
                <span className="timeline__message">{event.message}</span>
                <span className="timeline__time tabular">
                  {formatRelative(event.timestamps.ingestedAt)}
                </span>
              </div>
              <div className="timeline__meta">
                {oltNames.get(event.oltId) ?? event.oltId}
                {ponLabel ? ` · ${ponLabel}` : ""}
                {event.onuId !== null ? ` · ONU ${event.onuId}` : ""}
                {" · "}
                <span className="mono">{event.type}</span>
              </div>
              <div className="timeline__times">
                <span>
                  device:{" "}
                  {event.timestamps.deviceTime === null
                    ? "not reported"
                    : event.timestamps.quality === "untrusted"
                      ? `${formatDateTime(event.timestamps.deviceTime)} (suspect)`
                      : formatDateTime(event.timestamps.deviceTime)}
                </span>
                <span>ingested: {formatDateTime(event.timestamps.ingestedAt)}</span>
                {showEvidence ? (
                  <span className="mono timeline__evidence">{event.evidenceRef}</span>
                ) : null}
              </div>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
