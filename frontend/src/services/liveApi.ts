import { request } from "./api";
import { USE_MOCK_API } from "./client";

export interface StartedRun {
  runId: number;
  stream: "sse";
}

/** 202: the run exists, the collection is happening in the background. */
export async function startCollection(oltId: string): Promise<StartedRun> {
  if (USE_MOCK_API) {
    throw new Error("Live collection needs the API (set USE_MOCK_API = false).");
  }
  return request<StartedRun>(`/olts/${oltId}/collect`, { method: "POST", timeoutMs: 15_000 });
}

export interface LiveEvent {
  seq: number;
  kind: "start" | "info" | "cmd" | "out" | "error" | "end" | "truncated";
  data?: string;
  at: string;
}

/**
 * Subscribes to a run's output.
 *
 * EventSource sends Last-Event-ID automatically after a reconnect, so a dropped
 * connection resumes instead of replaying the whole run or losing the gap.
 * Cookie auth works because this is the same origin as the UI.
 */
export function streamRun(
  runId: number,
  onEvent: (event: LiveEvent) => void,
  onError: (message: string) => void,
): () => void {
  const source = new EventSource(`/api/v1/collection/runs/${runId}/stream`, {
    withCredentials: true,
  });

  const handle = (raw: MessageEvent<string>) => {
    try {
      onEvent(JSON.parse(raw.data) as LiveEvent);
    } catch {
      onError("unreadable event from the server");
    }
  };

  for (const kind of ["start", "info", "cmd", "out", "error", "end", "truncated"] as const) {
    source.addEventListener(kind, handle as EventListener);
  }

  source.onerror = () => {
    // EventSource retries by itself; only surface a message when it gives up.
    if (source.readyState === EventSource.CLOSED) {
      onError("live stream closed — the run may have finished");
    }
  };

  return () => source.close();
}
