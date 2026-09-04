import { Weapon } from "./Weapon.jsx";

export const FocusHud = ({ frame, focusPlayer }) => {
  if (!frame || !focusPlayer) return null;
  const p = frame.playerStates?.find((x) => x.steamId === focusPlayer);
  if (!p) return null;
  const sb = [
    ...(frame.ctScoreBoard || []),
    ...(frame.tScoreBoard || []),
  ].find((x) => x.steamId === focusPlayer);
  return (
    <div className={`focus-hud ${p.alive ? "" : "dead"}`}>
      <div className="focus-hud-name">{p.name}</div>
      <div className="focus-hud-stats">
        <span
          className={
            p.health > 50 ? "hp hp-high" : p.health > 25 ? "hp hp-mid" : "hp hp-low"
          }
        >
          {p.health} HP
        </span>
        {sb && (
          <span>
            {sb.kills}K / {sb.deaths}D / {sb.damage} DMG
          </span>
        )}
      </div>
      <div className="focus-hud-weapon">
        <Weapon weapon={p.weapon} />
      </div>
    </div>
  );
};