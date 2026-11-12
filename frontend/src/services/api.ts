/**
 * HTTP transport. One place that knows how to talk to the Go API.
 *
 * Everything above this file deals in typed promises; everything below is
 * fetch, headers, timeouts and the conversion of a bad response into an
 * ApiError that carries a kind an operator can act on.
 */

import type {
  DiagnosticResult,
  CollectionJob,
  Incident,
  NetworkEvent,
  Onu,
  OverviewResponse,
  Olt,
  Pon,
} from "../types/domain";

// Mutable so a deployment (or a test) can point the SPA at another host
// without a rebuild.
let baseUrl: string = import.meta.env.VITE_API_BASE ?? "/api/v1";

export function configureBaseUrl(url: string): void {
  baseUrl = url.replace(/\/$/, "");
}

export function currentBaseUrl(): string {
  return baseUrl;
}

export type ApiErrorKind = "network" | "timeout" | "http" | "parse" | "unauthorized";

export class ApiError extends Error {
  readonly status: number;
  readonly kind: ApiErrorKind;

  constructor(message: string, status: number, kind: ApiErrorKind) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.kind = kind;
  }
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  /** Bearer token. Never read from localStorage — see auth notes. */
  token?: string | null;
  timeoutMs?: number;
  signal?: AbortSignal;
}

const DEFAULT_TIMEOUT_MS = 10_000;

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, token, timeoutMs = DEFAULT_TIMEOUT_MS, signal } = options;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  // A caller-supplied signal must also be able to cancel the request.
  signal?.addEventListener("abort", () => controller.abort(), { once: true });

  let response: Response;
  try {
    response = await fetch(`${baseUrl}${path}`, {
      method,
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new ApiError(`Request to ${path} timed out after ${timeoutMs} ms`, 0, "timeout");
    }
    throw new ApiError(`Network error calling ${path}`, 0, "network");
  } finally {
    clearTimeout(timer);
  }

  if (response.status === 401 || response.status === 403) {
    throw new ApiError("Not authorised for this action", response.status, "unauthorized");
  }

  if (!response.ok) {
    // The body is either our own {"error":{"code","message"}} envelope or HTML
    // from a proxy. Showing the raw JSON in a dialog is how "knownhosts: key is
    // unknown" reached a user wrapped in braces.
    const text = await response.text().catch(() => "");
    let message = text || `HTTP ${response.status} from ${path}`;
    try {
      const body = JSON.parse(text) as { error?: { message?: string } };
      if (body.error?.message) message = body.error.message;
    } catch {
      // Not JSON: keep the text, it is the only evidence available.
    }
    throw new ApiError(message, response.status, "http");
  }

  if (response.status === 204) return undefined as T;

  try {
    return (await response.json()) as T;
  } catch {
    throw new ApiError(`Response from ${path} was not valid JSON`, response.status, "parse");
  }
}

/** The proposed contract from the design spec, typed end to end. */
export const api = {
  overview: () => request<OverviewResponse>("/dashboard/overview"),
  olts: () => request<Olt[]>("/olts"),
  olt: (id: string) => request<Olt>(`/olts/${id}`),
  pons: (oltId: string) => request<Pon[]>(`/olts/${oltId}/pons`),
  onus: (ponId: string) => request<Onu[]>(`/pons/${ponId}/onus`),
  onu: (id: string) => request<Onu>(`/onus/${id}`),
  events: () => request<NetworkEvent[]>("/events"),
  incidents: () => request<Incident[]>("/incidents"),
  incident: (id: string) => request<Incident>(`/incidents/${id}`),
  collectionJobs: () => request<CollectionJob[]>("/collection/jobs"),
  runDiagnostic: (oltId: string, commandId: string) =>
    request<DiagnosticResult>(`/olts/${oltId}/diagnostics`, {
      method: "POST",
      body: { commandId },
      token: null,
    }),
};
