import React from "react";
import { buyType } from "../utils/economy";

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
  mySteamId,
  myTeam,
  myTeamByRound,
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
          {killMarks.map((k, i) => {
            // Green = an enemy of mine died, red = a teammate of mine died;
            // without a known team fall back to the victim's side color.
            const markerClass = myTeam
              ? k.victimTeam === myTeam
                ? "us"
                : "them"
              : k.victimTeam === "CT"
                ? "ct"
                : "t";
            return (
              <div
                key={i}
                className={`tl-kill ${markerClass}`}
                style={{ left: `${(k.idx / max) * 100}%` }}
                title={`${k.attacker} killed ${k.victim} (${k.weapon})`}
              />
            );
          })}
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
          {myTeam ? (
            <>
              <span className="legend-kill-them">| enemy down</span>
              <span className="legend-kill-us">| teammate down</span>
            </>
          ) : (
            <span className="legend-kill">| kill</span>
          )}
          <span className="legend-bomb">| plant</span>
        </div>
      </div>
      <div className="round-buttons">
        {rounds.map((r, i) => {
          const ct = buyType(r.ctBuy);
          const t = buyType(r.tBuy);
          const team = myTeamByRound?.[r.round] || null;
          // Win/loss from my team's perspective (green/red); fall back to the
          // winner-side colors when my team for that round is unknown.
          const myWin = team && r.winner === team;
          const myLoss = team && r.winner && r.winner !== team;
          const title =
            ct || t
              ? `Round ${i}: CT ${ct ? ct.desc : "?"} · T ${t ? t.desc : "?"}` +
                (team ? ` · you played ${team}` : "")
              : undefined;
          return (
            <button
              key={i}
              className={[
                "round-button",
                i === currentRound ? "active" : "",
                myWin ? "my-win" : myLoss ? "my-loss" : "",
                !team && r.winner === "T" ? "t-win" : "",
                !team && r.winner === "CT" ? "ct-win" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              onClick={() => onSelect(i)}
              title={title}
            >
              <span className="round-num">{i}</span>
              <span className="round-buys">
                {team && (
                  <span className={`side-chip ${team.toLowerCase()}`}>
                    {team}
                  </span>
                )}
                {ct && (
                  <span className="buy-chip" style={{ background: ct.color }}>
                    {ct.short}
                  </span>
                )}
                {t && (
                  <span className="buy-chip" style={{ background: t.color }}>
                    {t.short}
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
};