export interface TabItem {
  id: string;
  label: string;
  /** Optional count or hint shown after the label. */
  badge?: string;
}

interface TabsProps {
  items: TabItem[];
  activeId: string;
  onChange: (id: string) => void;
  label: string;
}

/**
 * Tabs are buttons plus aria state, not decorative links. The panel they
 * control is rendered by the page, which keeps routing decisions out of here.
 */
export function Tabs({ items, activeId, onChange, label }: TabsProps) {
  return (
    <div className="tabs" role="tablist" aria-label={label}>
      {items.map((item) => {
        const isActive = item.id === activeId;
        return (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            className={isActive ? "tabs__tab tabs__tab--active" : "tabs__tab"}
            onClick={() => onChange(item.id)}
          >
            {item.label}
            {item.badge ? <span className="tabs__badge">{item.badge}</span> : null}
          </button>
        );
      })}
    </div>
  );
}
