import { TeamScoreBoard } from "./TeamScoreBoard";
import { buyType } from "../utils/economy";

export const ScoreBoard = ({
  frame,
  team,
  econ,
  onSelectPlayer,
  focusPlayer,
  mySteamId,
  mine,
}) => {
  const score = frame ? (team === "T" ? frame.tScore : frame.ctScore) : 0;
  const teamScores = frame
    ? team === "T"
        ? frame.tScoreBoard
        : frame.ctScoreBoard
    : [];
  const buy = econ ? buyType(econ.type) : null;
  const avgEquip =
    econ && typeof econ.avgEquip === "number"
      ? `$${(Math.round(econ.avgEquip / 100) / 10).toFixed(1)}k`
      : null;
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
      }}
    >
      <h2 style={{ margin: "0 0 8px" }}>
        {team} {score}
        {mine != null && (
          <span className={`team-pill ${mine ? "you" : "enemy"}`}>
            {mine ? "YOUR TEAM" : "ENEMY"}
          </span>
        )}
        {buy && (
          <span
            className="buy-pill"
            style={{ background: buy.color }}
            title={buy.desc}
          >
            {buy.label}
          </span>
        )}
        {avgEquip && (
          <span
            className="econ-value"
            title="average equipment value at freeze-time end"
          >
            {avgEquip}
          </span>
        )}
      </h2>
      <TeamScoreBoard
        teamScores={teamScores}
        econ={econ}
        onSelectPlayer={onSelectPlayer}
        focusPlayer={focusPlayer}
        mySteamId={mySteamId}
      />
    </div>
  );
};