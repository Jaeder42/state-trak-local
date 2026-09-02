import React, { useState } from "react";

export const FilterMenu = ({ filters, onToggle }) => {
  const [open, setOpen] = useState(false);

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
          <label>
            <input
              type="checkbox"
              checked={filters.health}
              onChange={() => onToggle("health")}
            />
            Health bars
          </label>
          <label>
            <input
              type="checkbox"
              checked={filters.names}
              onChange={() => onToggle("names")}
            />
            Player names
          </label>
        </div>
      )}
    </div>
  );
};
