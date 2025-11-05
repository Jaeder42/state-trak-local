import { TeamScoreBoard } from "./TeamScoreBoard";

export const ScoreBoard = ({ frame }) => {
    if (frame) {
        return (
            <div
                style={{
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    justifyContent: "center",
                    
                }}
            >
                <TeamScoreBoard teamScores={frame.tScoreBoard} />

                <TeamScoreBoard teamScores={frame.ctScoreBoard} />
            </div>
        );
    }
    return null;
};
