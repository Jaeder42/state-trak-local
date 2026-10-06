import React, { useEffect, useState } from "react";

// Keyboard shortcut reference — toggled by the "?" key or its button.

const SHORTCUTS = [
  ["Space", "play / pause"],
  ["← / →", "step one frame"],
  ["Shift + ← / →", "jump ±5 seconds"],
  [", / .", "previous / next round"],
  ["Home / End", "round start / end"],
  ["F11", "fullscreen"],
  ["Esc", "close panel · release focus cam"],
  ["Click player", "focus-follow camera"],
  ["Drag / wheel", "pan / zoom the map"],
  ["Click a timeline tick", "jump to that kill"],
];

export const ShortcutsHelp = () => {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (e) => {
      if (e.target.tagName === "INPUT") return;
      if (e.key === "?") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="filter-menu">
      <button
        className="filter-toggle"
        aria-label="Keyboard shortcuts"
        title="Keyboard shortcuts (?)"
        onClick={() => setOpen((o) => !o)}
      >
        ?
      </button>
      {open && (
        <div className="filter-panel shortcuts-panel">
          {SHORTCUTS.map(([keys, desc]) => (
            <div className="shortcut-row" key={keys}>
              <kbd>{keys}</kbd>
              <span>{desc}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};