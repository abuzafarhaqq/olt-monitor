import type {
  CollectionHealthResponse,
  CollectionJob,
  CollectionJobResult,
} from "../../types/domain";

export interface CollectionKpis {
  collectorsOnline: number;
  collectorsTotal: number;
  runningJobs: number;
  queuedJobs: number;
  failedJobs: number;
  partialJobs: number;
  unreachableOlts: number;
  staleSources: number;
}

export interface JobFilters {
  /** "" means every OLT. */
  oltId: string;
  /** "" means every result. */
  result: string;
}

export const EMPTY_FILTERS: JobFilters = { oltId: "", result: "" };

export function getCollectionKpis(data: CollectionHealthResponse): CollectionKpis {
  const jobs = data.jobs;

  return {
    collectorsOnline: data.collectors.filter((collector) => collector.state !== "stopped").length,
    collectorsTotal: data.collectors.length,
    runningJobs: data.collectors.reduce((sum, collector) => sum + collector.runningJobs, 0),
    queuedJobs: data.collectors.reduce((sum, collector) => sum + collector.queuedJobs, 0),
    failedJobs: jobs.filter((job) => job.result === "failed").length,
    partialJobs: jobs.filter((job) => job.result === "partial").length,
    unreachableOlts: data.olts.filter((olt) => !olt.reachable).length,
    staleSources: data.staleSources.length,
  };
}

/**
 * Filtering is a pure function of the filters, so the page can keep the whole
 * dataset in memory and still answer "what am I looking at" honestly.
 */
export function filterJobs(jobs: CollectionJob[], filters: JobFilters): CollectionJob[] {
  return jobs.filter((job) => {
    if (filters.oltId !== "" && job.oltId !== filters.oltId) return false;
    if (filters.result !== "" && job.result !== (filters.result as CollectionJobResult)) return false;
    return true;
  });
}

/** How many ONUs kept their last known state because collection told us nothing. */
export function getStatePreservedCount(jobs: CollectionJob[]): number {
  return jobs.reduce((sum, job) => sum + (job.stateLeftUnchanged ?? 0), 0);
}
