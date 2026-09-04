import { Weapon } from "./Weapon.jsx";

const WINDOW_FRAMES = 7 * 60; // keep kills visible for ~7 seconds

export const KillFeed = ({ kills, frames, index }) => {
  if (!kills?.length || !frames?.length) return null;
  const first = frames[0].frame;
  const visible = kills
    .map((k) => ({ ...k, idx: k.frame - first }))
    .filter((k) => k.idx <= index && index - k.idx < WINDOW_FRAMES);
  if (!visible.length) return null;
  return (
    <div className="kill-feed">
      {visible.map((k, i) => (
        <div className="kill-feed-entry" key={`${k.frame}-${k.victimSteamId}-${i}`}>
          <span className={`kf-name ${k.attackerTeam === "CT" ? "ct" : "t"}`}>
            {k.attacker || "World"}
          </span>
          <Weapon weapon={k.weapon} />
          {k.headshot && (
            <span className="kf-hs" title="Headshot">
              HS
            </span>
          )}
          {k.assister && <span className="kf-assist">+{k.assister}</span>}
          <span className={`kf-name ${k.victimTeam === "CT" ? "ct" : "t"}`}>
            {k.victim}
          </span>
        </div>
      ))}
    </div>
  );
};