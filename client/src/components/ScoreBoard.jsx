import { TeamScoreBoard } from "./TeamScoreBoard";

export const ScoreBoard = ({ frame, team, onSelectPlayer, focusPlayer }) => {
    if (frame) {
        const score = team === "T" ? frame.tScore : frame.ctScore;
        return (
            <div
                style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                }}
            >
                <h2 style={{ margin: "0 0 8px" }}>{team} {score}</h2>
                <TeamScoreBoard
                    teamScores={team === "T" ? frame.tScoreBoard : frame.ctScoreBoard}
                    onSelectPlayer={onSelectPlayer}
                    focusPlayer={focusPlayer}
                />
            </div>
        );
    }
    return null;
};
