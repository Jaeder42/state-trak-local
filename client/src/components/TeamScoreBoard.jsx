import { Weapon } from "./Weapon";

const MIN_ROWS = 5;

export const TeamScoreBoard = ({ teamScores, onSelectPlayer, focusPlayer }) => {
  const players = teamScores || [];
  const rows = [...players];
  while (rows.length < MIN_ROWS) {
    rows.push(null);
  }

  return (
    <div className="team-scoreboard">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Kills</th>
            <th>Assists</th>
            <th>Deaths</th>
            <th>MVP</th>
            <th>DMG</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((player, i) =>
            player ? (
              <tr key={player.steamId || player.name}>
                <td
                  className={`player-name ${
                    focusPlayer === player.steamId ? "focused" : ""
                  }`}
                  onClick={() => onSelectPlayer(player.steamId)}
                >
                  <span className="player-name-cell">
                    <Weapon weapon={player.weapon} />
                    {player.name}
                  </span>
                </td>
                <td>{player.kills}</td>
                <td>{player.assists}</td>
                <td>{player.deaths}</td>
                <td>{player.mvps}</td>
                <td>{player.damage}</td>
              </tr>
            ) : (
              <tr key={`empty-${i}`} className="empty-row">
                <td colSpan={6}>&nbsp;</td>
              </tr>
            )
          )}
        </tbody>
      </table>
    </div>
  );
};
