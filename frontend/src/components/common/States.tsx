/**
 * The three states every data screen will eventually need. Written now so that
 * Stage 2 can swap mock data for a fetch without inventing new markup.
 */

interface EmptyStateProps {
  title: string;
  description?: string;
}

export function EmptyState({ title, description }: EmptyStateProps) {
  return (
    <div className="state-block">
      <p className="state-block__title">{title}</p>
      {description ? <p className="state-block__description">{description}</p> : null}
    </div>
  );
}

interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = "Loading…" }: LoadingStateProps) {
  return (
    <div className="state-block" role="status" aria-live="polite">
      <p className="state-block__title">{label}</p>
    </div>
  );
}

interface ErrorStateProps {
  title?: string;
  description: string;
  onRetry?: () => void;
}

export function ErrorState({
  title = "Could not load this data",
  description,
  onRetry,
}: ErrorStateProps) {
  return (
    <div className="state-block state-block--error" role="alert">
      <p className="state-block__title">{title}</p>
      <p className="state-block__description">{description}</p>
      {onRetry ? (
        <button type="button" className="badge badge--info" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </div>
  );
}
