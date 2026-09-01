import React from "react";

export const Controls = ({ playing, onTogglePlay }) => {
  return (
    <button
      className="play-toggle"
      onClick={() => onTogglePlay(!playing)}
      title={playing ? "Pause" : "Play"}
    >
      {playing ? "❚❚" : "▶"}
    </button>
  );
};
