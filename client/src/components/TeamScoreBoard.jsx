import { Weapon } from "./Weapon";

const MIN_ROWS = 5;

export const TeamScoreBoard = ({
  teamScores,
  econ,
  onSelectPlayer,
  focusPlayer,
  mySteamId,
}) => {
  const players = teamScores || [];
  const rows = [...players];
  while (rows.length < MIN_ROWS) {
    rows.push(null);
  }

  // Per-player money from the round's economy snapshot — the bank after the
  // buy (freeze-time end). Demos parsed before the economy snapshot lack
  // it and show “–”.
  const money = (steamId) => {
    const p = econ?.players?.find((x) => x.steamId === steamId);
    if (!p) return null;
    return {
      bank: p.bank,
      title: `start $${p.startMoney} · spent $${p.spent} · equip $${p.equipValue} · bank $${p.bank}`,
    };
  };

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
            <th title="money left after the buy">$</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((player, i) =>
            player ? (
              <tr
                key={player.steamId || player.name}
                className={player.steamId === mySteamId ? "me" : ""}
              >
                <td
                  className={`player-name ${
                    focusPlayer === player.steamId ? "focused" : ""
                  }`}
                  onClick={() => onSelectPlayer(player.steamId)}
                >
                  <span className="player-name-cell">
                    <Weapon weapon={player.weapon} />
                    {player.name}
                    {player.steamId === mySteamId && (
                      <span className="you-tag">(you)</span>
                    )}
                  </span>
                </td>
                <td>{player.kills}</td>
                <td>{player.assists}</td>
                <td>{player.deaths}</td>
                <td>{player.mvps}</td>
                <td>{player.damage}</td>
                {(() => {
                  const m = money(player.steamId);
                  return (
                    <td className="money-cell" title={m ? m.title : undefined}>
                      {m ? `$${m.bank}` : "–"}
                    </td>
                  );
                })()}
              </tr>
            ) : (
              <tr key={`empty-${i}`} className="empty-row">
                <td colSpan={7}>&nbsp;</td>
              </tr>
            )
          )}
        </tbody>
      </table>
    </div>
  );
};
