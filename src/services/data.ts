/**
 * The one place a page asks for data.
 *
 * Mock and real sources expose the same functions, so a page never knows which
 * is running — and flipping USE_MOCK_API changes the whole estate at once
 * instead of one component at a time.
 */
import type { Onu, OnuDiagnostics, Olt, OverviewResponse } from "../types/domain";
import { api } from "./api";
import { USE_MOCK_API } from "./client";
import { MOCK_OVERVIEW } from "../mock/mockData";
import { ONU_DIAGNOSTICS } from "../mock/onuDiagnostics";

export interface DataSource {
  overview(): Promise<OverviewResponse>;
  olt(id: string): Promise<Olt | null>;
  onu(id: string): Promise<Onu | null>;
  diagnostics(id: string): Promise<OnuDiagnostics | null>;
}

// A real network round trip has latency; the mock should too, or loading states
// never get exercised during development.
const settle = <T>(value: T): Promise<T> =>
  new Promise((resolve) => setTimeout(() => resolve(value), 120));

export const mockSource: DataSource = {
  overview: () => settle(MOCK_OVERVIEW),
  olt: (id) => settle(MOCK_OVERVIEW.olts.find((olt) => olt.id === id) ?? null),
  onu: (id) => settle(MOCK_OVERVIEW.onus.find((onu) => onu.id === id) ?? null),
  diagnostics: (id) => settle(ONU_DIAGNOSTICS.get(id) ?? null),
};

export const apiSource: DataSource = {
  overview: () => api.overview(),
  olt: (id) => api.olt(id).catch(() => null),
  onu: (id) => api.onu(id).catch(() => null),
  // Optical history and evidence endpoints exist; the panel that shows them is
  // still fixture-backed, so this returns the mock for now and says so.
  diagnostics: (id) => settle(ONU_DIAGNOSTICS.get(id) ?? null),
};

export const dataSource: DataSource = USE_MOCK_API ? mockSource : apiSource;
