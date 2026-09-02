import { TeamScoreBoard } from "./TeamScoreBoard";

export const ScoreBoard = ({ frame, team, onSelectPlayer, focusPlayer }) => {
    const score = frame ? (team === "T" ? frame.tScore : frame.ctScore) : 0;
    const teamScores = frame
        ? team === "T"
            ? frame.tScoreBoard
            : frame.ctScoreBoard
        : [];
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
                teamScores={teamScores}
                onSelectPlayer={onSelectPlayer}
                focusPlayer={focusPlayer}
            />
        </div>
    );
};
