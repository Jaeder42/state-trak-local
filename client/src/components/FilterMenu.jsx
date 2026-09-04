import React, { useState } from "react";

export const FilterMenu = ({ filters, onToggle }) => {
  const [open, setOpen] = useState(false);

  const item = (key, label) => (
    <label>
      <input type="checkbox" checked={filters[key]} onChange={() => onToggle(key)} />
      {label}
    </label>
  );

  return (
    <div className="filter-menu">
      <button
        className="filter-toggle"
        onClick={() => setOpen((o) => !o)}
        title="Display filters"
      >
        ⚙
      </button>
      {open && (
        <div className="filter-panel">
          {item("names", "Player names")}
          {item("health", "Health bars")}
          {item("trails", "Player trails")}
          {item("theater", "Theater mode (hide scoreboard)")}
        </div>
      )}
    </div>
  );
};