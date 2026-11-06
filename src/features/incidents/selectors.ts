import type { Incident, IncidentState, NetworkEvent } from "../../types/domain";

export interface IncidentFilters {
  state: string;
  severity: string;
  oltId: string;
  query: string;
}

export const EMPTY_INCIDENT_FILTERS: IncidentFilters = {
  state: "",
  severity: "",
  oltId: "",
  query: "",
};

export function filterIncidents(incidents: Incident[], filters: IncidentFilters): Incident[] {
  const query = filters.query.trim().toLowerCase();

  return incidents
    .filter((incident) => {
      if (filters.state !== "" && incident.state !== filters.state) return false;
      if (filters.severity !== "" && incident.severity !== filters.severity) return false;
      if (filters.oltId !== "" && incident.oltId !== filters.oltId) return false;
      if (query !== "" && !`${incident.id} ${incident.title}`.toLowerCase().includes(query)) {
        return false;
      }
      return true;
    })
    .sort((a, b) => new Date(b.openedAt).getTime() - new Date(a.openedAt).getTime());
}

export function countByState(incidents: Incident[]): Record<IncidentState, number> {
  const counts: Record<IncidentState, number> = {
    new: 0,
    acknowledged: 0,
    investigating: 0,
    resolved: 0,
    closed: 0,
  };
  for (const incident of incidents) counts[incident.state] += 1;
  return counts;
}

export interface EventFilters {
  severity: string;
  oltId: string;
  type: string;
  query: string;
}

export const EMPTY_EVENT_FILTERS: EventFilters = {
  severity: "",
  oltId: "",
  type: "",
  query: "",
};

/** Newest first, duplicates preserved (§7: never blind-deduplicate). */
export function filterEvents(events: NetworkEvent[], filters: EventFilters): NetworkEvent[] {
  const query = filters.query.trim().toLowerCase();

  return events
    .filter((event) => {
      if (filters.severity !== "" && event.severity !== filters.severity) return false;
      if (filters.oltId !== "" && event.oltId !== filters.oltId) return false;
      if (filters.type !== "" && event.type !== filters.type) return false;
      if (query !== "" && !event.message.toLowerCase().includes(query)) return false;
      return true;
    })
    .sort(
      (a, b) =>
        new Date(b.timestamps.ingestedAt).getTime() -
        new Date(a.timestamps.ingestedAt).getTime(),
    );
}

export function getEventTypes(events: NetworkEvent[]): string[] {
  return [...new Set(events.map((event) => event.type))].sort();
}

/** Events correlated into an incident, oldest first — a timeline, not a list. */
export function getIncidentEvidence(incident: Incident, events: NetworkEvent[]): NetworkEvent[] {
  return events
    .filter((event) => incident.evidenceEventIds.includes(event.id))
    .sort(
      (a, b) =>
        new Date(a.timestamps.ingestedAt).getTime() -
        new Date(b.timestamps.ingestedAt).getTime(),
    );
}
