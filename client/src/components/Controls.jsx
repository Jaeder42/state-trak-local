import React from "react";

const SPEEDS = [0.25, 0.5, 1, 2, 4];

export const Controls = ({ playing, onTogglePlay, speed, onSpeedChange, children }) => {
  return (
    <div className="controls-bar">
      {children}
      <div className="speed-buttons">
        {SPEEDS.map((s) => (
          <button
            key={s}
            className={`speed-button ${speed === s ? "active" : ""}`}
            onClick={() => onSpeedChange(s)}
            title={`${s}× playback speed`}
            aria-label={`${s}× playback speed`}
          >
            {s}×
          </button>
        ))}
      </div>
      <button
        className="play-toggle"
        onClick={() => onTogglePlay(!playing)}
        title={playing ? "Pause (Space)" : "Play (Space)"}
        aria-label={playing ? "Pause" : "Play"}
      >
        {playing ? "❚❚" : "▶"}
      </button>
    </div>
  );
};