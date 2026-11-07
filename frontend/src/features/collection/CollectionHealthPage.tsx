import { useState } from "react";
import { DataTable, type Column } from "../../components/common/DataTable";
import { FilterBar, Select } from "../../components/common/FilterBar";
import { FreshnessIndicator } from "../../components/common/FreshnessIndicator";
import { JobResultBadge } from "../../components/common/JobResultBadge";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { MOCK_COLLECTION_HEALTH } from "../../mock/collectionHealth";
import type {
  CollectionJob,
  CollectionJobType,
  Collector,
  CollectorState,
  StaleSource,
} from "../../types/domain";
import { formatCount, formatDuration, formatRelative } from "../../utils/format";
import {
  EMPTY_FILTERS,
  filterJobs,
  getCollectionKpis,
  getStatePreservedCount,
  type JobFilters,
} from "./selectors";

const JOB_LABELS: Record<CollectionJobType, string> = {
  "onu-status": "ONU status",
  "onu-optical": "Optical diagnostics",
  "onu-basic-info": "ONU basic info",
  "syslog-onu-events": "Syslog: ONU events",
  "alarm-catalog": "Alarm catalog",
  "device-info": "Device info",
};

const COLLECTOR_TONES: Record<CollectorState, string> = {
  running: "badge--info",
  idle: "badge--online",
  degraded: "badge--warning",
  stopped: "badge--unknown",
};

/**
 * A failed collection is an absence of knowledge, not a device state change.
 * This page is where that distinction is enforced visibly.
 */
export function CollectionHealthPage() {
  const data = MOCK_COLLECTION_HEALTH;
  const kpis = getCollectionKpis(data);
  const [filters, setFilters] = useState<JobFilters>(EMPTY_FILTERS);

  const visibleJobs = filterJobs(data.jobs, filters);
  const statePreserved = getStatePreservedCount(data.jobs);
  const oltNames = new Map(data.olts.map((olt) => [olt.id, olt.name]));

  const collectorColumns: Column<Collector>[] = [
    {
      key: "name",
      header: "Collector",
      render: (collector) => (
        <span className="data-table__cell-strong mono">{collector.name}</span>
      ),
    },
    {
      key: "state",
      header: "State",
      render: (collector) => (
        <span className={`badge ${COLLECTOR_TONES[collector.state]}`}>{collector.state}</span>
      ),
    },
    { key: "running", header: "Running", align: "right", render: (c) => c.runningJobs },
    { key: "queued", header: "Queued", align: "right", render: (c) => c.queuedJobs },
    {
      key: "heartbeat",
      header: "Heartbeat",
      render: (collector) =>
        collector.heartbeatAt === null ? (
          <span className="cell-sub">never</span>
        ) : (
          <span className="tabular">{formatRelative(collector.heartbeatAt)}</span>
        ),
    },
  ];

  const jobColumns: Column<CollectionJob>[] = [
    {
      key: "olt",
      header: "OLT",
      render: (job) => oltNames.get(job.oltId) ?? job.oltId,
    },
    {
      key: "type",
      header: "Job",
      render: (job) => <span className="mono">{JOB_LABELS[job.type]}</span>,
    },
    {
      key: "lastRun",
      header: "Last run",
      render: (job) => (
        <FreshnessIndicator
          timestamps={{ deviceTime: null, ingestedAt: job.lastRunAt, quality: "trusted" }}
        />
      ),
    },
    {
      key: "duration",
      header: "Duration",
      align: "right",
      render: (job) => formatDuration(job.durationMs),
    },
    {
      key: "result",
      header: "Result",
      render: (job) => <JobResultBadge result={job.result} />,
    },
    {
      key: "onus",
      header: "ONU state",
      render: (job) =>
        job.stateLeftUnchanged === null ? (
          <span className="cell-sub">updated</span>
        ) : (
          <span className="cell-warning">left as last known ({job.stateLeftUnchanged})</span>
        ),
    },
    {
      key: "error",
      header: "Detail",
      render: (job) =>
        job.error === null ? (
          <span className="cell-sub">—</span>
        ) : (
          <span className="job-error">
            <span className={`badge ${job.error.kind === "parser" ? "badge--info" : "badge--warning"}`}>
              {job.error.kind}
            </span>
            <span className="job-error__text">{job.error.message}</span>
          </span>
        ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Collection Health"
        subtitle={`Snapshot generated ${formatRelative(data.generatedAt)} · ${formatCount(data.jobs.length)} jobs across ${data.olts.length} OLTs`}
        isMock
      />

      <section className="kpi-grid" aria-label="Collection metrics">
        <MetricCard
          label="Collectors"
          value={`${kpis.collectorsOnline}/${kpis.collectorsTotal}`}
          hint="not stopped"
          tone={kpis.collectorsOnline === kpis.collectorsTotal ? "online" : "warning"}
        />
        <MetricCard label="Running jobs" value={kpis.runningJobs} hint="in flight" tone="info" />
        <MetricCard label="Queued jobs" value={kpis.queuedJobs} hint="waiting for a worker" tone="warning" />
        <MetricCard label="Failed jobs" value={kpis.failedJobs} hint="last run per job" tone="warning" />
        <MetricCard label="Partial jobs" value={kpis.partialJobs} hint="answered, but incomplete" tone="warning" />
        <MetricCard label="Unreachable OLTs" value={kpis.unreachableOlts} hint="no management access" tone="offline" />
        <MetricCard label="Stale sources" value={kpis.staleSources} hint="older than the profile interval" tone="unknown" />
        <MetricCard
          label="ONU states preserved"
          value={statePreserved}
          hint="collection failed — state not rewritten"
          tone="investigating"
        />
      </section>

      <div className="callout" role="note">
        <strong>A failed collection never marks ONUs offline.</strong>{" "}
        {formatCount(statePreserved)} ONU states on this page kept their last
        authoritative value because the run returned nothing trustworthy. They stay
        <span className="badge badge--unknown" style={{ margin: "0 4px" }}>
          Unknown
        </span>
        or last known — not
        <span className="badge badge--offline" style={{ margin: "0 4px" }}>
          Offline
        </span>
        .
      </div>

      <section className="card" style={{ marginBottom: "var(--space-4)" }} aria-label="Collectors">
        <div className="card__header">
          <h2 className="card__title">Active collectors</h2>
          <span className="card__aside">heartbeat and queue depth</span>
        </div>
        <div className="card__body card__body--flush">
          <DataTable
            columns={collectorColumns}
            rows={data.collectors}
            getRowKey={(collector) => collector.id}
            caption="Active collectors"
            emptyMessage="No collector is running."
          />
        </div>
      </section>

      <section className="card" aria-label="Collection jobs">
        <div className="card__header">
          <h2 className="card__title">Collection jobs</h2>
          <span className="card__aside">last run per OLT and job type</span>
        </div>

        <div className="card__body">
          <FilterBar
            summary={`${visibleJobs.length} of ${data.jobs.length} jobs`}
            onReset={() => setFilters(EMPTY_FILTERS)}
          >
            <Select
              id="filter-olt"
              label="OLT"
              value={filters.oltId}
              onChange={(oltId) => setFilters((current) => ({ ...current, oltId }))}
              options={[
                { value: "", label: "All OLTs" },
                ...data.olts.map((olt) => ({ value: olt.id, label: olt.name })),
              ]}
            />
            <Select
              id="filter-result"
              label="Result"
              value={filters.result}
              onChange={(result) => setFilters((current) => ({ ...current, result }))}
              options={[
                { value: "", label: "All results" },
                { value: "successful", label: "Success" },
                { value: "partial", label: "Partial" },
                { value: "failed", label: "Failed" },
                { value: "skipped", label: "Skipped" },
              ]}
            />
          </FilterBar>
        </div>

        <div className="card__body card__body--flush">
          <DataTable
            columns={jobColumns}
            rows={visibleJobs}
            getRowKey={(job) => job.id}
            caption="Collection jobs"
            emptyMessage="No job matches these filters."
          />
        </div>
      </section>

      <section className="card" style={{ marginTop: "var(--space-4)" }} aria-label="Stale sources">
        <div className="card__header">
          <h2 className="card__title">Stale data sources</h2>
          <span className="card__aside">judged against the collection interval, not a fixed age</span>
        </div>
        <div className="card__body card__body--flush">
          <DataTable
            columns={[
              {
                key: "olt",
                header: "OLT",
                render: (source: StaleSource) => oltNames.get(source.oltId) ?? source.oltId,
              },
              {
                key: "lastSuccess",
                header: "Last success",
                render: (source) =>
                  source.lastSuccessAt === null ? "never" : formatRelative(source.lastSuccessAt),
              },
              {
                key: "age",
                header: "Age / interval",
                align: "right",
                render: (source) => `${source.ageMinutes}m / ${source.intervalMinutes}m`,
              },
              { key: "reason", header: "Reason", render: (source) => source.reason },
            ]}
            rows={data.staleSources}
            getRowKey={(source) => source.oltId}
            caption="Stale data sources"
            emptyMessage="No stale source."
          />
        </div>
      </section>
    </>
  );
}
