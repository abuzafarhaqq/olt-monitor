import { useEffect, type ReactNode } from "react";

interface DetailDrawerProps {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  /** "wide" for a terminal: 460 px forces a horizontal scroll on device output. */
  size?: "default" | "wide";
}

/**
 * Side panel for details and short forms. Escape closes it, because a drawer
 * with no keyboard exit traps people who do not use a mouse.
 */
export function DetailDrawer({ title, subtitle, onClose, children, footer, size = "default" }: DetailDrawerProps) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    // Cleanup: without this, every open/close cycle leaks a listener.
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <>
      <button type="button" className="drawer-scrim" aria-label="Close panel" onClick={onClose} />
      <aside
        className={size === "wide" ? "drawer drawer--wide" : "drawer"}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header className="drawer__header">
          <div>
            <h2 className="drawer__title">{title}</h2>
            {subtitle ? <p className="drawer__subtitle">{subtitle}</p> : null}
          </div>
          <button type="button" className="drawer__close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>
        <div className="drawer__body">{children}</div>
        {footer ? <footer className="drawer__footer">{footer}</footer> : null}
      </aside>
    </>
  );
}
