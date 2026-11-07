import type { DiagnosticResult } from "../types/domain";
import { api } from "./api";
import { runDiagnostic } from "./mockBackend";

/**
 * The single switch between the in-memory fixture and the Go API.
 *
 * Flip USE_MOCK_API to false once `/api/v1` answers; nothing else in the app
 * changes. Keeping the decision in one constant is what stops a "temporary"
 * mock call from surviving into production in twelve different components.
 */
// Driven by the environment so one script (app.sh) can start the same build in
// either mode. VITE_USE_MOCK_API=false talks to the Go API.
export const USE_MOCK_API = (import.meta.env.VITE_USE_MOCK_API ?? "true") !== "false";

export async function executeDiagnostic(
  oltId: string,
  commandId: string,
): Promise<DiagnosticResult> {
  return USE_MOCK_API ? runDiagnostic(oltId, commandId) : api.runDiagnostic(oltId, commandId);
}
