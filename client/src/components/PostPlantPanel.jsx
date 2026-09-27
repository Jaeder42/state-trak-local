import React, { useMemo, useState } from "react";
import { MiniRadar } from "./MiniRadar.jsx";
import { mapDisplayName } from "../maps/config";
import {
  buildPostPlantNarrative,
  resolveSides,
} from "../utils/postplantText";

const T_COLOR = "#e6f13d";
const CT_COLOR = "#68a3e5";
const WIN_COLOR = "#4caf50";
const LOSS_COLOR = "#f44336";

const avg = (vals) =>
  vals.length ? vals.reduce((a, b) => a + b, 0) / vals.length : null;
const fmtU = (v) => (v == null ? "–" : `${Math.round(v)}u`);
const fmtS = (v) => (v == null ? "–" : `${v.toFixed(1)}s`);

const OUTCOMES = {
  defused: { label: "DEFUSED", cls: "ct", title: "CT defused the bomb" },
  exploded: { label: "EXPLODED", cls: "t", title: "The bomb exploded" },
  eliminated: {
    label: "ELIMINATED",
    cls: "t",
    title: "Ts killed every CT after the plant",
  },
};

const outcomePill = (outcome) => {
  const o = OUTCOMES[outcome];
  if (!o) return <span className="pp-outcome">{outcome || "?"}</span>;
  return (
    <span className={`pp-outcome ${o.cls}`} title={o.title}>
      {o.label}
    </span>
  );
};

// metric row helper: average of round[key] within a group
const groupMetric = (group, key) =>
  avg(group.map((r) => r[key]).filter((v) => v != null));

const MetricRow = ({ label, groupA, groupB, keyName, fmt }) => (
  <tr>
    <td>{label}</td>
    <td>{fmt(groupMetric(groupA, keyName))}</td>
    <td>{fmt(groupMetric(groupB, keyName))}</td>
  </tr>
);

// One side's narrative block (good + bad pattern bullets).
const NarrativeSide = ({ title, side }) => (
  <div className="pp-narrative-side">
    <h5>
      {title}
      {side.headline && (
        <span className="pp-narrative-headline"> — {side.headline}</span>
      )}
    </h5>
    {!side.headline ? (
      <p className="pp-note">No planted rounds on this side.</p>
    ) : side.good.length || side.bad.length ? (
      <ul className="pp-narrative-list">
        {side.good.map((g, i) => (
          <li key={i} className="pp-good">
            <span className="pp-mark">✓</span> {g}
          </li>
        ))}
        {side.bad.map((b, i) => (
          <li key={i} className="pp-bad">
            <span className="pp-mark">✗</span> {b}
          </li>
        ))}
      </ul>
    ) : (
      <p className="pp-note">No clear pattern across these rounds.</p>
    )}
  </div>
);

export const PostPlantPanel = ({
  data,
  mapName,
  mySteamId,
  players,
  roundSummaries,
  onClose,
  onWatch,
}) => {
  const rounds = useMemo(() => data.rounds || [], [data]);
  const summary = data.summary || {};
  const [chosenSteamId, setChosenSteamId] = useState(mySteamId || "");

  // Which side the chosen player's team was on per planted round, and the
  // narrative derived from it.
  const sides = useMemo(
    () => resolveSides(rounds, roundSummaries || [], chosenSteamId),
    [rounds, roundSummaries, chosenSteamId],
  );  const narrative = useMemo(
    () => buildPostPlantNarrative(rounds, sides, roundSummaries || []),
    [rounds, sides, roundSummaries],
  );
  const playerOptions = useMemo(() => {
    const opts = [];
    const seen = new Set();
    const me = (players || []).find((p) => p.steamId === mySteamId);
    if (mySteamId) {
      seen.add(mySteamId);
      opts.push({
        steamId: mySteamId,
        name: me?.name ? `You — ${me.name}` : "You",
      });
    }
    (players || []).forEach((p) => {
      if (!p.steamId || seen.has(p.steamId)) return;
      seen.add(p.steamId);
      opts.push({ steamId: p.steamId, name: p.name || p.steamId });
    });
    return opts;
  }, [players, mySteamId]);

  const held = rounds.filter((r) => r.winner === "T"); // plant defended
  const lost = rounds.filter((r) => r.winner === "CT"); // defused
  const retakes = rounds.filter((r) => r.ctEntryTime != null);
  const retakeWon = retakes.filter((r) => r.winner === "CT");
  const retakeFailed = retakes.filter((r) => r.winner === "T");

  // dot helpers — the chosen player gets a white halo
  const dot = (p, color, extra = {}) => ({
    x: p.position.x,
    y: p.position.y,
    color,
    hollow: p.alive === false,
    ring: chosenSteamId && p.steamId === chosenSteamId,
    ...extra,
  });

  // aggregate radars across all rounds, dots colored by round outcome
  const tSetupDots = rounds.flatMap((r) =>
    (r.tSetup || []).map((p) =>
      dot(p, r.winner === "T" ? WIN_COLOR : LOSS_COLOR),
    ),
  );
  const plantBombs = rounds.map((r) => ({ x: r.bombPos.x, y: r.bombPos.y }));
  const ctEntryDots = retakes.flatMap((r) =>
    (r.ctEntry || []).map((p) =>
      dot(p, r.winner === "CT" ? WIN_COLOR : LOSS_COLOR),
    ),
  );

  return (
    <div className="analysis-overlay" onClick={onClose}>
      <div className="analysis-panel postplant-panel" onClick={(e) => e.stopPropagation()}>
        <div className="analysis-header">
          <h3>
            Post-plant positioning · {mapDisplayName(mapName)} ·{" "}
            {summary.planted ?? rounds.length} planted rounds
          </h3>
          <button className="analysis-close" onClick={onClose} title="Close">
            ×
          </button>
        </div>
        <p className="analysis-sub">
          Setup sampled 5s after the plant (dead players outline at where they
          fell); movement measured +5s→+15s; CT “entry” is the first CT within
          500u of the bomb. Phantom plants (planted after the round ended) are
          excluded.
        </p>

        {rounds.length === 0 ? (
          <p className="pp-empty">No planted rounds in this demo.</p>
        ) : (
          <>
            <div className="pp-narrative">
              <div className="pp-narrative-head">
                <h4>Team patterns</h4>
                <select
                  className="pp-player-select"
                  value={chosenSteamId}
                  onChange={(e) => setChosenSteamId(e.target.value)}
                  title="Choose whose team to analyze"
                >
                  {playerOptions.map((o) => (
                    <option key={o.steamId} value={o.steamId}>
                      {o.name}
                    </option>
                  ))}
                </select>
              </div>
              <NarrativeSide title="Planting (T)" side={narrative.t} />
              <NarrativeSide title="Retaking (CT)" side={narrative.ct} />
              {narrative.unknown > 0 && (
                <p className="pp-note">
                  {narrative.unknown} planted round
                  {narrative.unknown === 1 ? "" : "s"} where this player’s side
                  is unknown (no roster/snapshot data).
                </p>
              )}
            </div>

            <div className="pp-summary-grid">
              <table className="analysis-table">
                <thead>
                  <tr>
                    <th>T post-plant</th>
                    <th>held ({held.length})</th>
                    <th>defused ({lost.length})</th>
                  </tr>
                </thead>
                <tbody>
                  <MetricRow
                    label="setup spread"
                    groupA={held}
                    groupB={lost}
                    keyName="tSpread"
                    fmt={fmtU}
                  />
                  <MetricRow
                    label="dist to bomb"
                    groupA={held}
                    groupB={lost}
                    keyName="tAvgBombDist"
                    fmt={fmtU}
                  />
                  <MetricRow
                    label="movement +5→+15s"
                    groupA={held}
                    groupB={lost}
                    keyName="tMovement"
                    fmt={fmtU}
                  />
                </tbody>
              </table>
              <table className="analysis-table">
                <thead>
                  <tr>
                    <th>CT retake</th>
                    <th>won ({retakeWon.length})</th>
                    <th>failed ({retakeFailed.length})</th>
                  </tr>
                </thead>
                <tbody>
                  <MetricRow
                    label="time to entry"
                    groupA={retakeWon}
                    groupB={retakeFailed}
                    keyName="ctEntryTime"
                    fmt={fmtS}
                  />
                  <MetricRow
                    label="entry spread"
                    groupA={retakeWon}
                    groupB={retakeFailed}
                    keyName="ctEntrySpread"
                    fmt={fmtU}
                  />
                  <tr>
                    <td>no retake at all</td>
                    <td colSpan={2}>
                      {summary.noRetake ?? 0} of {held.length} held rounds — CTs
                      never reached the bomb
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div className="pp-radars">
              <figure className="pp-radar-fig">
                <MiniRadar
                  mapName={mapName}
                  dots={tSetupDots}
                  bombs={plantBombs}
                  size={190}
                />
                <figcaption>
                  T setups — <span className="pp-win">held</span> /{" "}
                  <span className="pp-loss">defused</span>
                </figcaption>
              </figure>
              <figure className="pp-radar-fig">
                <MiniRadar
                  mapName={mapName}
                  dots={ctEntryDots}
                  bombs={plantBombs}
                  size={190}
                />
                <figcaption>
                  CT entries — <span className="pp-win">defused</span> /{" "}
                  <span className="pp-loss">failed</span>
                </figcaption>
              </figure>
            </div>

            <table className="analysis-table pp-rounds-table">
              <thead>
                <tr>
                  <th>R#</th>
                  <th>Outcome</th>
                  <th>Plant</th>
                  <th>Alive@plant</th>
                  <th>T spread</th>
                  <th>T bomb</th>
                  <th>T move</th>
                  <th>CT entry</th>
                  <th>CT spread</th>
                  <th>Setup 5s after plant</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {rounds.map((r) => {
                  const side = sides[r.round];
                  const setupDots = [
                    ...(r.tSetup || []).map((p) => dot(p, T_COLOR)),
                    ...(r.ctSetup || []).map((p) =>
                      dot(p, CT_COLOR, { r: 2.5 }),
                    ),
                  ];
                  return (
                    <tr key={r.round}>
                      <td>
                        {r.round}
                        {side && (
                          <span
                            className={`side-chip ${side.toLowerCase()}`}
                            title={`chosen player played ${side}`}
                          >
                            {side}
                          </span>
                        )}
                      </td>
                      <td>{outcomePill(r.outcome)}</td>
                      <td>{fmtS(r.plantAt)}</td>
                      <td>
                        {r.tAliveAtPlant}v{r.ctAliveAtPlant}
                      </td>
                      <td>{fmtU(r.tSpread)}</td>
                      <td>{fmtU(r.tAvgBombDist)}</td>
                      <td>{fmtU(r.tMovement)}</td>
                      <td>
                        {r.ctEntryTime == null ? (
                          <span className="pp-noretake" title="no CT reached the bomb">
                            no retake
                          </span>
                        ) : (
                          <>
                            {fmtS(r.ctEntryTime)}
                            <span className="pp-entry-n">
                              {" "}×{(r.ctEntry || []).length}
                            </span>
                          </>
                        )}
                      </td>
                      <td>{fmtU(r.ctEntrySpread)}</td>
                      <td>
                        <MiniRadar
                          mapName={mapName}
                          dots={setupDots}
                          bombs={[{ x: r.bombPos.x, y: r.bombPos.y }]}
                          size={150}
                        />
                      </td>
                      <td>
                        <button
                          className="pp-watch"
                          onClick={() => onWatch(r.round, r.plantFrameIndex)}
                          title="jump to the plant in the playback"
                        >
                          ▶
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <div className="analysis-footer">
              outcomes: {summary.defused ?? 0} defused · {summary.exploded ?? 0}{" "}
              exploded · {summary.eliminated ?? 0} eliminated post-plant
            </div>
          </>
        )}
      </div>
    </div>
  );
};