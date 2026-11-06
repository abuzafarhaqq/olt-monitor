/**
 * Client-side export. No server round trip: the data is already in memory.
 *
 * CSV is quoted per RFC 4180 — a title containing a comma or a quote must not
 * break the file an operator opens in a spreadsheet.
 */

export type ExportRow = Record<string, string | number | null>;

function escapeCell(value: string | number | null): string {
  const text = value === null ? "" : String(value);
  return /[",\n\r]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}

export function toCsv(rows: ExportRow[]): string {
  if (rows.length === 0) return "";
  const headers = Object.keys(rows[0]);
  const lines = [
    headers.join(","),
    ...rows.map((row) => headers.map((header) => escapeCell(row[header])).join(",")),
  ];
  return `${lines.join("\n")}\n`;
}

export function toJson(rows: ExportRow[]): string {
  return `${JSON.stringify(rows, null, 2)}\n`;
}

/** Browser-only: creates an object URL, clicks a temporary anchor, cleans up. */
export function downloadFile(filename: string, content: string, mime: string): void {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  // Without revoking, the blob stays alive until the tab closes.
  URL.revokeObjectURL(url);
}
