import { PageHeader } from "../../components/common/PageHeader";

interface PlaceholderPageProps {
  title: string;
  stage: number;
}

/**
 * Routes exist from Stage 1 so navigation can be tested end to end, but an
 * unimplemented page says so instead of showing an empty table that looks broken.
 */
export function PlaceholderPage({ title, stage }: PlaceholderPageProps) {
  return (
    <>
      <PageHeader title={title} subtitle="Not implemented yet — route and layout only." />
      <div className="placeholder">
        <span className="placeholder__stage">
          {stage > 0 ? `Planned for stage ${stage}` : "Unknown route"}
        </span>
        <p className="placeholder__text">
          This page is wired into the router and the sidebar so navigation can be
          verified now. Its content arrives in a later stage, built from the same
          design tokens and reusable components as the Overview.
        </p>
      </div>
    </>
  );
}
