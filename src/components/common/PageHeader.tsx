import type { ReactNode } from "react";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  /**
   * Renders the MOCK DATA flag. Any screen fed by mock data must set this:
   * mock numbers must never be readable as live measurements (§12).
   */
  isMock?: boolean;
  actions?: ReactNode;
}

export function PageHeader({ title, subtitle, isMock = false, actions }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div>
        <h1 className="page-header__title">{title}</h1>
        {subtitle ? <p className="page-header__subtitle">{subtitle}</p> : null}
      </div>
      <div className="page-header__actions">
        {actions}
        {isMock ? <span className="mock-flag">MOCK DATA — not live</span> : null}
      </div>
    </header>
  );
}
