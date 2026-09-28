import React, { useState } from "react";

export const FilterMenu = ({
  filters,
  onToggle,
  mySteamId,
  onMySteamIdChange,
  myTeamActive,
}) => {
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
          <div className="steam-id-setting">
            <span className="steam-id-label">My Steam ID</span>
            <input
              className="steam-id-input"
              type="text"
              value={mySteamId}
              onChange={(e) => onMySteamIdChange(e.target.value)}
              placeholder="76561198…"
              spellCheck={false}
            />
            <span className={`steam-id-status ${myTeamActive ? "ok" : ""}`}>
              {myTeamActive ? "✓ in this demo" : "not in this demo"}
            </span>
          </div>
        </div>
      )}
    </div>
  );
};