import { useState } from "react";
import { TimeSeriesChart } from "../../components/charts/TimeSeriesChart";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FilterBar, Select } from "../../components/common/FilterBar";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { DETAILED_RETENTION_DAYS, REPORTS, getReport } from "../../constants/reports";
import { HISTORY, HISTORY_RANGE } from "../../mock/history";
import { downloadFile, toCsv, toJson, type ExportRow } from "../../utils/export";
import {
  buildRows,
  compare,
  daysBetween,
  formatValue,
  getSeries,
  pointsInRange,
  previousRange,
  shiftDays,
} from "./selectors";

const PRESETS = [
  { id: "7d", label: "7 days", days: 7 },
  { id: "30d", label: "30 days", days: 30 },
  { id: "90d", label: "90 days", days: 90 },
];

export function ReportsPage() {
  const [reportId, setReportId] = useState("onu-availability");
  const [to, setTo] = useState(HISTORY_RANGE.to);
  const [from, setFrom] = useState(shiftDays(HISTORY_RANGE.to, -29));
  const [compareOn, setCompareOn] = useState(true);

  const report = getReport(reportId);
  const series = getSeries(HISTORY, reportId);
  const current = pointsInRange(series?.points ?? [], from, to);
  const previousWindow = previousRange(from, to);
  const previous = compareOn
    ? pointsInRange(series?.points ?? [], previousWindow.from, previousWindow.to)
    : [];
  const comparison = compare(current, previous, report);
  const rows = buildRows(current, compareOn ? previous : null);

  const applyPreset = (days: number) => {
    setTo(HISTORY_RANGE.to);
    setFrom(shiftDays(HISTORY_RANGE.to, -(days - 1)));
  };

  const activePreset = PRESETS.find(
    (preset) => daysBetween(from, to) === preset.days && to === HISTORY_RANGE.to,
  );

  const exportRows: ExportRow[] = rows.map((row) => ({
    date: row.date,
    [report.id]: row.value,
    ...(compareOn ? { previous_period: row.previous } : {}),
  }));

  const filename = `${report.id}_${from}_${to}`;

  const columns: Column<ExportRow>[] = [
    { key: "date", header: "Date", render: (row) => <span className="tabular">{row.date}</span> },
    {
      key: "value",
      header: report.label,
      align: "right",
      render: (row) => formatValue(row.value as number, report),
    },
    ...(compareOn
      ? [
          {
            key: "previous",
            header: "Previous period",
            align: "right" as const,
            render: (row: ExportRow) =>
              formatValue(row.previous === null ? null : (row.previous as number), report),
          },
        ]
      : []),
  ];

  const deltaTone =
    comparison.improved === null ? "neutral" : comparison.improved ? "online" : "offline";

  return (
    <>
      <PageHeader
        title="History & Reports"
        subtitle={`Daily aggregates from ${HISTORY_RANGE.from} to ${HISTORY_RANGE.to} · detailed measurements retained ${DETAILED_RETENTION_DAYS} days`}
        isMock
        actions={
          <>
            <button
              type="button"
              className="btn"
              onClick={() =>
                downloadFile(`${filename}.csv`, toCsv(exportRows), "text/csv;charset=utf-8")
              }
            >
              Export CSV
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => downloadFile(`${filename}.json`, toJson(exportRows), "application/json")}
            >
              Export JSON
            </button>
          </>
        }
      />

      <section className="card" aria-label="Report filters">
        <div className="card__body">
          <FilterBar summary={`${current.length} days selected`}>
            <Select
              id="report-type"
              label="Report"
              value={reportId}
              onChange={setReportId}
              options={REPORTS.map((candidate) => ({ value: candidate.id, label: candidate.label }))}
            />

            <div className="preset-group" role="group" aria-label="Range preset">
              {PRESETS.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className={activePreset?.id === preset.id ? "preset preset--active" : "preset"}
                  onClick={() => applyPreset(preset.days)}
                >
                  {preset.label}
                </button>
              ))}
            </div>

            <div className="filter-field">
              <label className="filter-field__label" htmlFor="report-from">
                From
              </label>
              <input
                id="report-from"
                type="date"
                className="filter-field__control"
                value={from}
                min={HISTORY_RANGE.from}
                max={to}
                onChange={(event) => setFrom(event.target.value)}
              />
            </div>

            <div className="filter-field">
              <label className="filter-field__label" htmlFor="report-to">
                To
              </label>
              <input
                id="report-to"
                type="date"
                className="filter-field__control"
                value={to}
                min={from}
                max={HISTORY_RANGE.to}
                onChange={(event) => setTo(event.target.value)}
              />
            </div>

            <label className="checkbox">
              <input
                type="checkbox"
                checked={compareOn}
                onChange={(event) => setCompareOn(event.target.checked)}
              />
              Compare with previous {daysBetween(from, to)} days
            </label>
          </FilterBar>
        </div>
      </section>

      <section className="kpi-grid" aria-label="Report summary">
        <MetricCard
          label={`${report.label} (${report.aggregation === "avg" ? "average" : "total"})`}
          value={formatValue(comparison.current, report)}
          hint={report.unit === "%" ? "percent" : report.unit}
          tone="info"
        />
        {compareOn ? (
          <>
            <MetricCard
              label="Previous period"
              value={formatValue(comparison.previous, report)}
              hint={`${previousWindow.from} → ${previousWindow.to}`}
            />
            <MetricCard
              label="Change"
              value={
                comparison.delta === null
                  ? "—"
                  : `${comparison.delta > 0 ? "+" : ""}${comparison.delta.toFixed(report.precision)}`
              }
              hint={
                comparison.deltaPct === null
                  ? "no baseline"
                  : `${comparison.deltaPct > 0 ? "+" : ""}${comparison.deltaPct.toFixed(1)}% vs previous`
              }
              tone={deltaTone}
            />
          </>
        ) : null}
      </section>

      <div className="callout" role="note">
        <strong>{report.description}</strong> Comparison uses the immediately preceding period of
        the same length ({daysBetween(from, to)} days), never a different window.
      </div>

      <section className="card" aria-label="Report chart">
        <div className="card__header">
          <h2 className="card__title">{report.label}</h2>
          <span className="card__aside">solid = selected range · dashed = previous period</span>
        </div>
        <div className="card__body">
          <TimeSeriesChart
            current={current}
            previous={compareOn ? previous : undefined}
            unit={report.unit}
            precision={report.precision}
          />
        </div>
      </section>

      <section className="card" aria-label="Report rows">
        <div className="card__body card__body--flush">
          <DataTable
            columns={columns}
            rows={rows.slice().reverse()}
            getRowKey={(row) => String(row.date)}
            caption={`${report.label} by day`}
            emptyMessage="No data in this range."
          />
        </div>
      </section>
    </>
  );
}
