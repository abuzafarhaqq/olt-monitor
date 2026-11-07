import { request } from "./api";
import { USE_MOCK_API } from "./client";

export interface RunSummary {
  id: number;
  oltId: string;
  oltName: string;
  jobType: string;
  result: "successful" | "partial" | "failed" | "skipped";
  startedAt: string;
  finishedAt: string | null;
  durationMs: number | null;
  errorMessage: string | null;
  evidenceCount: number;
}

export interface Evidence {
  id: string;
  command: string;
  capturedAt: string;
  parserVersion: string;
  completeness: number;
  body: string;
}

export async function listRuns(limit = 50): Promise<RunSummary[]> {
  if (USE_MOCK_API) return [];
  return request<RunSummary[]>(`/collection/runs?limit=${limit}`);
}

export async function runEvidence(id: number): Promise<Evidence[]> {
  if (USE_MOCK_API) return [];
  const payload = await request<{ id: number; evidence: Evidence[] }>(`/collection/runs/${id}`);
  return payload.evidence;
}
