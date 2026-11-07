import { useState } from "react";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import { EventTimeline } from "../../components/common/EventTimeline";
import { IncidentStateBadge } from "../../components/common/IncidentStateBadge";
import { ALLOWED_TRANSITIONS, TRANSITION_LABELS } from "../../constants/incidentWorkflow";
import type { Incident, IncidentNote, IncidentState, NetworkEvent, Olt } from "../../types/domain";
import { formatDateTime, formatRelative } from "../../utils/format";
import { getIncidentEvidence } from "./selectors";

interface IncidentDrawerProps {
  incident: Incident;
  incidents: Incident[];
  events: NetworkEvent[];
  olts: Olt[];
  onClose: () => void;
  onTransition: (id: string, next: IncidentState) => void;
  onAddNote: (id: string, note: IncidentNote) => void;
}

/**
 * Incident case view. Transitions are offered from ALLOWED_TRANSITIONS only —
 * the UI cannot invent a move the workflow forbids. Everything here is local
 * state: there is no API yet, and the panel says so rather than pretending.
 */
export function IncidentDrawer({
  incident,
  incidents,
  events,
  olts,
  onClose,
  onTransition,
  onAddNote,
}: IncidentDrawerProps) {
  const [draft, setDraft] = useState("");
  const olt = olts.find((candidate) => candidate.id === incident.oltId);
  const evidence = getIncidentEvidence(incident, events);
  const related = incidents.filter(
    (candidate) => candidate.id !== incident.id && candidate.oltId === incident.oltId,
  );
  const nextStates = ALLOWED_TRANSITIONS[incident.state];

  const submitNote = () => {
    const text = draft.trim();
    if (text === "") return;
    onAddNote(incident.id, {
      id: `note-${Date.now()}`,
      author: "NOC Operator",
      at: new Date().toISOString(),
      text,
    });
    setDraft("");
  };

  return (
    <DetailDrawer
      title={incident.title}
      subtitle={`${incident.id} · opened ${formatRelative(incident.openedAt)}`}
      onClose={onClose}
      footer={
        <>
          <span className="drawer__hint">State changes are local to this session</span>
          {nextStates.map((next) => (
            <button
              key={next}
              type="button"
              className={next === "resolved" ? "btn" : "btn btn--primary"}
              onClick={() => onTransition(incident.id, next)}
            >
              {TRANSITION_LABELS[next]}
            </button>
          ))}
        </>
      }
    >
      <div className="drawer-section">
        <div className="drawer-section__row">
          <IncidentStateBadge state={incident.state} />
          <span className={`badge ${incident.severity === "critical" ? "badge--offline" : "badge--warning"}`}>
            {incident.severity}
          </span>
          <span className="cell-sub">
            {olt?.name ?? incident.oltId}
            {incident.ponId ? ` · ${incident.ponId.split("-").pop()}` : ""}
          </span>
        </div>
      </div>

      {incident.hypothesis ? (
        <div className="callout callout--hypothesis" role="note">
          <strong>Hypothesis — not a confirmed root cause.</strong> {incident.hypothesis}
        </div>
      ) : null}

      <h3 className="drawer-section__title">Evidence ({evidence.length})</h3>
      <EventTimeline events={evidence} olts={olts} />

      <h3 className="drawer-section__title">Notes ({incident.notes.length})</h3>
      {incident.notes.length === 0 ? (
        <p className="cell-sub">No note yet.</p>
      ) : (
        <ul className="note-list">
          {incident.notes.map((note) => (
            <li className="note" key={note.id}>
              <div className="note__head">
                <strong>{note.author}</strong>
                <span className="cell-sub">{formatDateTime(note.at)}</span>
              </div>
              <p className="note__text">{note.text}</p>
            </li>
          ))}
        </ul>
      )}

      <div className="note-form">
        <label className="form__label" htmlFor="incident-note">
          Add note
        </label>
        <textarea
          id="incident-note"
          className="form__control note-form__input"
          rows={3}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="What did you check, and what did it show?"
        />
        <button type="button" className="btn btn--small" onClick={submitNote} disabled={draft.trim() === ""}>
          Add note
        </button>
      </div>

      {related.length > 0 ? (
        <>
          <h3 className="drawer-section__title">Other cases on this OLT</h3>
          <ul className="note-list">
            {related.map((candidate) => (
              <li className="note" key={candidate.id}>
                <div className="note__head">
                  <strong>{candidate.id}</strong>
                  <IncidentStateBadge state={candidate.state} />
                </div>
                <p className="note__text">{candidate.title}</p>
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </DetailDrawer>
  );
}
