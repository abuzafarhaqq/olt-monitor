/**
 * Live updates.
 *
 * The real implementation is an EventSource on GET /api/v1/stream that, after
 * every reconnect, fetches a fresh snapshot — because events missed while
 * disconnected are not replayed by SSE, and a reconnected client that only
 * patches what arrives next would silently hold stale state.
 *
 * Until the API exists this module emits a scripted feed with the same shape.
 */

export type LiveSeverity = "info" | "warning" | "critical";

export interface LiveEvent {
  id: string;
  at: string;
  severity: LiveSeverity;
  message: string;
}

export type LiveSink = (event: LiveEvent) => void;

const SCRIPT: Omit<LiveEvent, "id" | "at">[] = [
  { severity: "info", message: "Collection cycle completed on EPON-OLT-01" },
  { severity: "warning", message: "ONU 7 on epon 0/2 deregistered" },
  { severity: "critical", message: "ONU unreachable on GPON-OLT-03 — Telnet timeout" },
  { severity: "info", message: "Optical sample stored for 48 ONUs" },
  { severity: "warning", message: "Partial optical collection on GPON-OLT-02" },
];

/**
 * Returns an unsubscribe function. Always keep it — a component that
 * subscribes without cleaning up keeps a timer alive after unmount.
 */
export function subscribeToLiveEvents(sink: LiveSink, intervalMs = 12_000): () => void {
  let index = 0;
  let stopped = false;

  const emit = () => {
    if (stopped) return;
    const entry = SCRIPT[index % SCRIPT.length];
    index += 1;
    sink({
      id: `live-${index}`,
      at: new Date().toISOString(),
      severity: entry.severity,
      message: entry.message,
    });
  };

  const timer = setInterval(emit, intervalMs);
  return () => {
    stopped = true;
    clearInterval(timer);
  };
}
