import { ScoreBoard } from "./ScoreBoard";

export const ScoreBoardPanel = ({ frame, onSelectPlayer, focusPlayer }) => {
  return (
    <div className="scoreboard-panel">
      <ScoreBoard
        frame={frame}
        team="T"
        onSelectPlayer={onSelectPlayer}
        focusPlayer={focusPlayer}
      />
      <ScoreBoard
        frame={frame}
        team="CT"
        onSelectPlayer={onSelectPlayer}
        focusPlayer={focusPlayer}
      />
    </div>
  );
};
