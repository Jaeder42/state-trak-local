import React from "react";

export const RoundSelector = ({
  rounds,
  currentRound,
  onSelect,
  index,
  max,
  onIndexChange,
}) => {
  if (!rounds || !rounds.length) return null;
  return (
    <div className="round-selector">
      <input
        className="progress-bar"
        type="range"
        min="0"
        max={max}
        value={index}
        onChange={(e) => onIndexChange(Number(e.target.value))}
      />
      <div className="round-buttons">
        {rounds.map((r, i) => (
          <button
            key={i}
            className={`round-button ${i === currentRound ? "active" : ""} ${
              r.winner === "T" ? "t-win" : r.winner === "CT" ? "ct-win" : ""
            }`}
            onClick={() => onSelect(i)}
          >
            {i}
          </button>
        ))}
      </div>
    </div>
  );
};
