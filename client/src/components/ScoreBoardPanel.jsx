import { ScoreBoard } from "./ScoreBoard";

export const ScoreBoardPanel = ({
  frame,
  onSelectPlayer,
  focusPlayer,
  economy,
  mySteamId,
  myTeam,
}) => {
  const sideEcon = (side) => (economy ? economy[side.toLowerCase()] : null);
  // Orient the scoreboard around the team I played on: my team on top,
  // the enemy below. Teams swap at halftime, so both orderings occur.
  const first = myTeam || "T";
  const second = first === "T" ? "CT" : "T";
  // mine: null = I'm not in this demo (no pill), true = my team, false = enemy
  const side = (team) => (myTeam ? team === myTeam : null);
  return (
    <div className="scoreboard-panel">
      <ScoreBoard
        frame={frame}
        team={first}
        econ={sideEcon(first)}
        onSelectPlayer={onSelectPlayer}
        focusPlayer={focusPlayer}
        mySteamId={mySteamId}
        mine={side(first)}
      />
      <ScoreBoard
        frame={frame}
        team={second}
        econ={sideEcon(second)}
        onSelectPlayer={onSelectPlayer}
        focusPlayer={focusPlayer}
        mySteamId={mySteamId}
        mine={side(second)}
      />
    </div>
  );
};