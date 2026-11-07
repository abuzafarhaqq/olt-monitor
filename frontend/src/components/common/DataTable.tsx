import type { ReactNode } from "react";
import { EmptyState } from "./States";

export interface Column<T> {
  /** Stable identity for React's key — never the row index. */
  key: string;
  header: string;
  align?: "left" | "right";
  /** How to draw this cell from the row. */
  render: (row: T) => ReactNode;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[];
  /** A table needs a real identity per row, so getRowKey is required. */
  getRowKey: (row: T) => string;
  emptyMessage?: string;
  caption?: string;
}

/**
 * Generic over T: one table component serves OLTs today and 25,000 ONUs later.
 * `<T,>` in a .tsx file is the type-parameter list — the comma tells the parser
 * this is a generic arrow function, not a JSX tag.
 */
export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  emptyMessage = "Nothing to show.",
  caption,
}: DataTableProps<T>) {
  if (rows.length === 0) {
    return <EmptyState title="No rows" description={emptyMessage} />;
  }

  return (
    // A wide diagnostic table scrolls inside its card instead of pushing the
    // whole page sideways on a phone.
    <div className="table-scroll">
    <table className="data-table">
      {caption ? <caption className="visually-hidden">{caption}</caption> : null}
      <thead>
        <tr>
          {columns.map((column) => (
            <th
              key={column.key}
              scope="col"
              className={column.align === "right" ? "align-right" : undefined}
            >
              {column.header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={getRowKey(row)}>
            {columns.map((column) => (
              <td
                key={column.key}
                className={column.align === "right" ? "align-right tabular" : undefined}
              >
                {column.render(row)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
    </div>
  );
}
