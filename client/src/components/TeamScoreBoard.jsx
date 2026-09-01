import { Weapon } from "./Weapon";

export const TeamScoreBoard = ({ teamScores, onSelectPlayer, focusPlayer }) => {
  if (teamScores) {
    return (
      <div style={{border: '1px solid white', margin: '10px', padding: '10px', borderRadius: '10px'}}>
        <table>
          <tr>
            <th>Name</th>
            <th>Kills</th>
            <th>Assists</th>
            <th>Deaths</th>
            <th>MVP</th>
            <th>DMG</th>
          </tr>
          {teamScores.map((player) => (
            <tr key={player.name}>
              <td
                className={`player-name ${focusPlayer === player.steamId ? "focused" : ""}`}
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
          ))}
        </table>
      </div>
    );
  }
  return null;
};
