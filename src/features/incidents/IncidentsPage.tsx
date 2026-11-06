import { useState } from "react";
import { DataTable, type Column } from "../../components/common/DataTable";
import { EventTimeline } from "../../components/common/EventTimeline";
import { FilterBar, Select, TextInput } from "../../components/common/FilterBar";
import { IncidentStateBadge } from "../../components/common/IncidentStateBadge";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { Tabs } from "../../components/common/Tabs";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import type { Incident, IncidentNote, IncidentState } from "../../types/domain";
import { formatRelative } from "../../utils/format";
import { IncidentDrawer } from "./IncidentDrawer";
import {
  EMPTY_EVENT_FILTERS,
  EMPTY_INCIDENT_FILTERS,
  countByState,
  filterEvents,
  filterIncidents,
  getEventTypes,
  type EventFilters,
  type IncidentFilters,
} from "./selectors";

export function IncidentsPage() {
  const data = MOCK_OVERVIEW;

  // Local copy: there is no write API, so state lives in this session only.
  const [incidents, setIncidents] = useState<Incident[]>(data.incidents);
  const [openId, setOpenId] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [tab, setTab] = useState("incidents");
  const [incidentFilters, setIncidentFilters] = useState<IncidentFilters>(EMPTY_INCIDENT_FILTERS);
  const [eventFilters, setEventFilters] = useState<EventFilters>(EMPTY_EVENT_FILTERS);

  const visibleIncidents = filterIncidents(incidents, incidentFilters);
  const visibleEvents = filterEvents(data.events, eventFilters);
  const counts = countByState(incidents);
  const selected = incidents.find((incident) => incident.id === openId) ?? null;

  const transition = (id: string, next: IncidentState) => {
    setIncidents((list) =>
      list.map((incident) => (incident.id === id ? { ...incident, state: next } : incident)),
    );
    setDirty(true);
  };

  const addNote = (id: string, note: IncidentNote) => {
    setIncidents((list) =>
      list.map((incident) =>
        incident.id === id ? { ...incident, notes: [...incident.notes, note] } : incident,
      ),
    );
    setDirty(true);
  };

  const columns: Column<Incident>[] = [
    {
      key: "id",
      header: "Incident",
      render: (incident) => (
        <div>
          <span className="mono">{incident.id}</span>
          <div className="cell-sub">{incident.title}</div>
        </div>
      ),
    },
    {
      key: "state",
      header: "State",
      render: (incident) => <IncidentStateBadge state={incident.state} />,
    },
    {
      key: "severity",
      header: "Severity",
      render: (incident) => (
        <span
          className={`badge ${incident.severity === "critical" ? "badge--offline" : "badge--warning"}`}
        >
          {incident.severity}
        </span>
      ),
    },
    {
      key: "device",
      header: "OLT / PON",
      render: (incident) => (
        <div>
          {data.olts.find((olt) => olt.id === incident.oltId)?.name ?? incident.oltId}
          <div className="cell-sub mono">
            {incident.ponId === null ? "device-level" : incident.ponId.split("-").pop()}
          </div>
        </div>
      ),
    },
    {
      key: "notes",
      header: "Notes",
      align: "right",
      render: (incident) => incident.notes.length,
    },
    {
      key: "opened",
      header: "Opened",
      render: (incident) => (
        <span className="tabular">{formatRelative(incident.openedAt)}</span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (incident) => (
        <button type="button" className="btn btn--small" onClick={() => setOpenId(incident.id)}>
          Open
        </button>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Incidents & Events"
        subtitle="Events are observations; incidents are cases built from them."
        isMock
      />

      {dirty ? (
        <div className="callout" role="status">
          <strong>Local session only.</strong> State changes and notes on this page are not
          saved — there is no write API yet, and reloading discards them.
        </div>
      ) : null}

      <section className="kpi-grid" aria-label="Incident states">
        <MetricCard label="New" value={counts.new} tone="info" />
        <MetricCard label="Acknowledged" value={counts.acknowledged} tone="investigating" />
        <MetricCard label="Investigating" value={counts.investigating} tone="investigating" />
        <MetricCard label="Resolved" value={counts.resolved} tone="online" />
        <MetricCard label="Closed" value={counts.closed} tone="unknown" />
        <MetricCard label="Events" value={data.events.length} hint="duplicates preserved" />
      </section>

      <Tabs
        items={[
          { id: "incidents", label: "Incidents", badge: String(incidents.length) },
          { id: "events", label: "Events", badge: String(data.events.length) },
        ]}
        activeId={tab}
        onChange={setTab}
        label="Incidents and events"
      />

      {tab === "incidents" ? (
        <section className="card" aria-label="Incidents">
          <div className="card__body">
            <FilterBar
              summary={`${visibleIncidents.length} of ${incidents.length} incidents`}
              onReset={() => setIncidentFilters(EMPTY_INCIDENT_FILTERS)}
            >
              <TextInput
                id="incident-search"
                label="Search"
                value={incidentFilters.query}
                placeholder="id or title"
                onChange={(query) => setIncidentFilters((current) => ({ ...current, query }))}
              />
              <Select
                id="incident-state"
                label="State"
                value={incidentFilters.state}
                onChange={(state) => setIncidentFilters((current) => ({ ...current, state }))}
                options={[
                  { value: "", label: "All" },
                  { value: "new", label: "New" },
                  { value: "acknowledged", label: "Acknowledged" },
                  { value: "investigating", label: "Investigating" },
                  { value: "resolved", label: "Resolved" },
                  { value: "closed", label: "Closed" },
                ]}
              />
              <Select
                id="incident-severity"
                label="Severity"
                value={incidentFilters.severity}
                onChange={(severity) => setIncidentFilters((current) => ({ ...current, severity }))}
                options={[
                  { value: "", label: "All" },
                  { value: "critical", label: "Critical" },
                  { value: "warning", label: "Warning" },
                  { value: "info", label: "Info" },
                ]}
              />
              <Select
                id="incident-olt"
                label="OLT"
                value={incidentFilters.oltId}
                onChange={(oltId) => setIncidentFilters((current) => ({ ...current, oltId }))}
                options={[
                  { value: "", label: "All OLTs" },
                  ...data.olts.map((olt) => ({ value: olt.id, label: olt.name })),
                ]}
              />
            </FilterBar>
          </div>
          <div className="card__body card__body--flush">
            <DataTable
              columns={columns}
              rows={visibleIncidents}
              getRowKey={(incident) => incident.id}
              caption="Incidents"
              emptyMessage="No incident matches these filters."
            />
          </div>
        </section>
      ) : (
        <section className="card" aria-label="Events">
          <div className="card__body">
            <FilterBar
              summary={`${visibleEvents.length} of ${data.events.length} events`}
              onReset={() => setEventFilters(EMPTY_EVENT_FILTERS)}
            >
              <TextInput
                id="event-search"
                label="Search"
                value={eventFilters.query}
                placeholder="message text"
                onChange={(query) => setEventFilters((current) => ({ ...current, query }))}
              />
              <Select
                id="event-severity"
                label="Severity"
                value={eventFilters.severity}
                onChange={(severity) => setEventFilters((current) => ({ ...current, severity }))}
                options={[
                  { value: "", label: "All" },
                  { value: "critical", label: "Critical" },
                  { value: "warning", label: "Warning" },
                  { value: "info", label: "Info" },
                ]}
              />
              <Select
                id="event-olt"
                label="OLT"
                value={eventFilters.oltId}
                onChange={(oltId) => setEventFilters((current) => ({ ...current, oltId }))}
                options={[
                  { value: "", label: "All OLTs" },
                  ...data.olts.map((olt) => ({ value: olt.id, label: olt.name })),
                ]}
              />
              <Select
                id="event-type"
                label="Type"
                value={eventFilters.type}
                onChange={(type) => setEventFilters((current) => ({ ...current, type }))}
                options={[
                  { value: "", label: "All types" },
                  ...getEventTypes(data.events).map((type) => ({ value: type, label: type })),
                ]}
              />
            </FilterBar>
          </div>
          <div className="card__body">
            <EventTimeline events={visibleEvents} olts={data.olts} />
          </div>
        </section>
      )}

      {selected !== null ? (
        <IncidentDrawer
          incident={selected}
          incidents={incidents}
          events={data.events}
          olts={data.olts}
          onClose={() => setOpenId(null)}
          onTransition={transition}
          onAddNote={addNote}
        />
      ) : null}
    </>
  );
}
