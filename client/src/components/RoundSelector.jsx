import React from "react";

const fmt = (secs) => {
  secs = Math.max(0, Math.floor(secs));
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
};

const BOMB_TIMER = 40; // CS2 bomb: 40 seconds

export const RoundSelector = ({
  rounds,
  currentRound,
  onSelect,
  output,
  index,
  onIndexChange,
}) => {
  const frames = output?.frames;
  if (!rounds || !rounds.length || !frames?.length) return null;

  const max = frames.length - 1;
  const first = frames[0];
  const frame = frames[Math.min(index, max)];

  const roundTime = frame.time - first.time;
  const totalTime = frames[max].time - first.time;

  const plantIndex = frames.findIndex((f) => f.bombState?.planted);
  let bombLeft = null;
  if (plantIndex >= 0 && index >= plantIndex) {
    const left = BOMB_TIMER - (frame.time - frames[plantIndex].time);
    if (left > 0) bombLeft = left;
  }

  const killMarks = (output.kills || [])
    .map((k) => ({ ...k, idx: k.frame - first.frame }))
    .filter((k) => k.idx >= 0 && k.idx <= max);

  return (
    <div className="round-selector">
      <div className="timeline-wrap">
        <input
          className="progress-bar"
          type="range"
          min="0"
          max={max}
          value={Math.min(index, max)}
          onChange={(e) => onIndexChange(Number(e.target.value))}
        />
        <div className="timeline-markers">
          {killMarks.map((k, i) => (
            <div
              key={i}
              className={`tl-kill ${k.victimTeam === "CT" ? "ct" : "t"}`}
              style={{ left: `${(k.idx / max) * 100}%` }}
              title={`${k.attacker} killed ${k.victim} (${k.weapon})`}
            />
          ))}
          {plantIndex >= 0 && (
            <div
              className="tl-bomb"
              style={{ left: `${(plantIndex / max) * 100}%` }}
              title="Bomb planted"
            />
          )}
        </div>
      </div>
      <div className="timeline-info">
        <span className="clock">
          {fmt(roundTime)} / {fmt(totalTime)}
        </span>
        {frame.phase === "LIVE" && <span className="phase-pill live">LIVE</span>}
        {bombLeft != null && (
          <span className={`bomb-pill ${bombLeft <= 10 ? "urgent" : ""}`}>
            💣 {fmt(bombLeft)}
          </span>
        )}
        <div className="legend">
          <span className="legend-ct">■ CT</span>
          <span className="legend-t">■ T</span>
          <span className="legend-kill">| kill</span>
          <span className="legend-bomb">| plant</span>
        </div>
      </div>
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