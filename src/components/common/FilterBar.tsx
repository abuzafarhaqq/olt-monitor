import type { ReactNode } from "react";

export interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps {
  id: string;
  label: string;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
}

/**
 * A controlled <select>: React owns the value, the DOM only reports intent.
 * The id/label pair makes the control reachable by screen readers and by a
 * click on the text.
 */
export function Select({ id, label, value, options, onChange }: SelectProps) {
  return (
    <div className="filter-field">
      <label className="filter-field__label" htmlFor={id}>
        {label}
      </label>
      <select
        id={id}
        className="filter-field__control"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

interface FilterBarProps {
  children: ReactNode;
  /** Right-aligned summary text, e.g. "12 of 18 jobs". */
  summary?: string;
  onReset?: () => void;
}

export function FilterBar({ children, summary, onReset }: FilterBarProps) {
  return (
    <div className="filter-bar" role="group" aria-label="Filters">
      {children}
      {summary ? <span className="filter-bar__summary">{summary}</span> : null}
      {onReset ? (
        <button type="button" className="filter-bar__reset" onClick={onReset}>
          Reset
        </button>
      ) : null}
    </div>
  );
}
