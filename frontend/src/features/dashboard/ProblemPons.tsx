import type { ProblemPon } from "./selectors";

interface ProblemPonsProps {
  problems: ProblemPon[];
}

/**
 * PONs ranked by impact, not by a single metric: forty offline ONUs matter more
 * than one weak RX reading. The score lives in selectors.ts so the ranking rule
 * is reviewable in one place.
 */
export function ProblemPons({ problems }: ProblemPonsProps) {
  if (problems.length === 0) {
    return <p className="card__body">No PON has a problem with fresh data.</p>;
  }

  return (
    <div className="problem-list">
      {problems.map((problem) => (
        <div className="problem-row" key={problem.pon.id}>
          <div>
            <div className="problem-row__name">{problem.pon.name}</div>
            <div className="problem-row__meta">{problem.oltName}</div>
          </div>
          <div className="problem-row__scores">
            <span>
              <span className="problem-row__score-value">{problem.offline}</span>
              offline
            </span>
            <span>
              <span className="problem-row__score-value">{problem.lowOptical}</span>
              low RX
            </span>
            <span>
              <span className="problem-row__score-value">{problem.dyingGasp}</span>
              gasp
            </span>
            <span>
              <span className="problem-row__score-value">{problem.flapping}</span>
              flap
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}
