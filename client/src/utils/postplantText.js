// Narrative post-plant analysis for one player's team, generated
// deterministically from the post-plant analysis data (GET /demos/:id/postplant).
//
// buildPostPlantNarrative() splits the planted rounds by which side the
// chosen player's team was on and surfaces good and bad patterns from the
// metrics: setup spread / distance to bomb / movement vs outcome on the T
// side, retake entry timing and grouping vs outcome on the CT side, plus
// trade balance and uncontested plants (with buy-type context — an eco save
// is discipline, an uncontested full buy is a giveaway).
//
// With small samples this deliberately stays conservative: bullets only fire
// on clear differences or absolute thresholds, and the text says "in this
// demo" implicitly. No bullet is invented to fill space.

const u = (v) => (v == null ? "?" : `${Math.round(v)}u`);
const s = (v) => (v == null ? "?" : `${v.toFixed(1)}s`);
const list = (rs) => rs.map((r) => r.round).join(", ");
const pct = (a, b) => (b ? Math.round((a / b) * 100) : 0);

const avg = (vals) => {
  const v = vals.filter((x) => x != null);
  return v.length ? v.reduce((a, b) => a + b, 0) / v.length : null;
};
const ravg = (group, key) => avg(group.map((r) => r[key]));

// thresholds — world units / seconds
const SPREAD_DIFF = 120; // meaningful spread difference between outcome groups
const DIST_CLOSE = 600; // avg setup distance considered "close to the bomb"
const DIST_FAR = 800; // avg setup distance considered "too far"
const MOVE_HOLD = 250; // avg movement (+5s→+15s) considered "holding"
const MOVE_PUSH = 400; // avg movement considered "pushing/peeking"
const ENTRY_RUSH = 8; // entry inside this many seconds is a rushed retake
const ENTRY_DIFF = 3; // meaningful entry-timing difference between groups

// resolveSides figures out which side the player's team was on for each
// planted round: primarily from the roster steam ids in the round summaries,
// falling back to the player appearing in the post-plant position snapshots.
// Gaps inherit the nearest known round's side.
export const resolveSides = (ppRounds, summaries, steamId) => {
  const sides = {};
  summaries.forEach((r) => {
    if ((r.ctSteamIds || []).includes(steamId)) sides[r.round] = "CT";
    else if ((r.tSteamIds || []).includes(steamId)) sides[r.round] = "T";
  });
  ppRounds.forEach((r) => {
    if (sides[r.round]) return;
    const t = (r.tSetup || []).some((p) => p.steamId === steamId);
    const ct =
      (r.ctSetup || []).some((p) => p.steamId === steamId) ||
      (r.ctEntry || []).some((p) => p.steamId === steamId);
    if (t) sides[r.round] = "T";
    else if (ct) sides[r.round] = "CT";
  });
  // gap-fill from the nearest known round over the full range of planted
  // rounds (sides swap at most at halftime, so the nearest known side is the
  // best guess for a round without roster/snapshot data)
  const nums = Object.keys(sides)
    .map(Number)
    .sort((a, b) => a - b);
  if (nums.length) {
    const firstKnown = nums[0];
    const all = [...new Set([...ppRounds.map((r) => r.round), ...nums])].sort(
      (a, b) => a - b,
    );
    let last = null;
    all.forEach((n) => {
      if (sides[n]) {
        last = sides[n];
      } else {
        sides[n] = n < firstKnown ? sides[firstKnown] : last;
      }
    });
  }
  return sides;
};

// tNarrative analyzes the rounds where the chosen player's team planted.
const tNarrative = (rounds, buyFor) => {
  const out = { headline: "", good: [], bad: [] };
  if (!rounds.length) return out;

  const held = rounds.filter((r) => r.winner === "T");
  const lost = rounds.filter((r) => r.winner === "CT");
  out.headline = `Held ${held.length} of ${rounds.length} plants (${pct(held.length, rounds.length)}%).`;

  const heldSpread = ravg(held, "tSpread");
  const lostSpread = ravg(lost, "tSpread");
  const heldDist = ravg(held, "tAvgBombDist");
  const lostDist = ravg(lost, "tAvgBombDist");
  const heldMove = ravg(held, "tMovement");
  const lostMove = ravg(lost, "tMovement");

  // ---- good patterns ----
  if (heldSpread != null && lostSpread != null && heldSpread <= lostSpread - SPREAD_DIFF) {
    out.good.push(
      `Tighter setups held: ≈${u(heldSpread)} spread in won rounds vs ${u(lostSpread)} when defused — the compact crossfire works.`,
    );
  }
  if (heldDist != null && lostDist != null && heldDist <= lostDist - SPREAD_DIFF && heldDist < DIST_CLOSE) {
    out.good.push(
      `Staying close to the C4 paid off: ≈${u(heldDist)} from the bomb in held rounds (${u(lostDist)} when defused).`,
    );
  }
  if (heldMove != null && heldMove < MOVE_HOLD && (lostMove == null || lostMove >= heldMove + SPREAD_DIFF)) {
    out.good.push(
      `You held your ground after the plant in the wins (only ${u(heldMove)} of movement 5→15s).`,
    );
  }
  if (held.length >= 2) {
    const ctDeaths = held.reduce((a, r) => a + r.postDeathsCT, 0);
    const tDeaths = held.reduce((a, r) => a + r.postDeathsT, 0);
    if (ctDeaths > tDeaths) {
      out.good.push(
        `Post-plant fights went your way in holds: ${ctDeaths} CT deaths traded for ${tDeaths} of yours.`,
      );
    }
  }
  if (held.length >= 2) {
    const model = held
      .filter((r) => r.tSpread != null)
      .sort((a, b) => (a.tSpread + (a.tAvgBombDist ?? 1500)) - (b.tSpread + (b.tAvgBombDist ?? 1500)))[0];
    if (model) {
      out.good.push(
        `Round ${model.round} is the model hold: ${u(model.tSpread)} spread, ${u(model.tAvgBombDist)} from the bomb${model.tMovement != null && model.tMovement < MOVE_HOLD ? ", barely moving" : ""}.`,
      );
    }
  }
  if (rounds.length === 1 && held.length === 1) {
    const r = held[0];
    out.good.push(
      `Round ${r.round}: held${r.tSpread != null ? ` with a ${u(r.tSpread)} spread` : ""}${r.tAvgBombDist != null && r.tAvgBombDist < DIST_CLOSE ? `, close to the bomb (${u(r.tAvgBombDist)})` : ""}.`,
    );
  }

  // ---- bad patterns ----
  if (lost.length) {
    if (lostDist != null && lostDist >= DIST_FAR) {
      out.bad.push(
        `Defused rounds averaged ${u(lostDist)} from the bomb — too far to contest a defuse (rounds ${list(lost)}).`,
      );
    } else if (lostDist != null && heldDist != null && lostDist >= heldDist + SPREAD_DIFF) {
      out.bad.push(
        `You set up farther from the C4 in lost rounds (${u(lostDist)} vs ${u(heldDist)} in holds).`,
      );
    }
    if (lostMove != null && lostMove >= MOVE_PUSH) {
      out.bad.push(
        `Too much movement after the plant when it mattered (${u(lostMove)} avg 5→15s in defused rounds) — pushing instead of holding angles.`,
      );
    } else if (lostMove != null && heldMove != null && lostMove >= heldMove + SPREAD_DIFF) {
      out.bad.push(
        `The defused rounds were the ones where you moved most (${u(lostMove)} vs ${u(heldMove)} in holds) — position discipline slipped.`,
      );
    }
    const noSetup = lost.filter(
      (r) => (r.tSetup || []).length === 0 || (r.tSetup || []).every((p) => !p.alive),
    );
    if (noSetup.length) {
      out.bad.push(
        `In ${noSetup.length === 1 ? `round ${noSetup[0].round}` : `rounds ${list(noSetup)}`} nobody was alive 5s after the plant — the site was handed over.`,
      );
    }
    const tDeaths = lost.reduce((a, r) => a + r.postDeathsT, 0);
    const ctDeaths = lost.reduce((a, r) => a + r.postDeathsCT, 0);
    if (tDeaths >= ctDeaths + 3) {
      out.bad.push(
        `Defused rounds were one-sided fights: ${tDeaths} of you died for ${ctDeaths} CT${ctDeaths === 1 ? "" : "s"} — the retake trades went against you.`,
      );
    }
  }
  return out;
};

// ctNarrative analyzes the rounds where the chosen player's team defended a
// plant (retakes).
const ctNarrative = (rounds, buyFor) => {
  const out = { headline: "", good: [], bad: [] };
  if (!rounds.length) return out;

  const won = rounds.filter((r) => r.winner === "CT");
  const lost = rounds.filter((r) => r.winner === "T");
  out.headline = `Faced ${rounds.length} plants, defused ${won.length} (${pct(won.length, rounds.length)}%).`;

  const wonEntries = won.filter((r) => r.ctEntryTime != null);
  const lostEntries = lost.filter((r) => r.ctEntryTime != null);
  const wonTime = ravg(wonEntries, "ctEntryTime");
  const lostTime = ravg(lostEntries, "ctEntryTime");
  const wonGroup = avg(wonEntries.map((r) => (r.ctEntry || []).length));
  const lostGroup = avg(lostEntries.map((r) => (r.ctEntry || []).length));

  // ---- good patterns ----
  if (wonTime != null && lostTime != null && wonTime >= lostTime + ENTRY_DIFF) {
    out.good.push(
      `Patient retakes worked: ~${s(wonTime)} from plant to first contact in defused rounds vs ${s(lostTime)} in failed ones — the extra seconds buy teammates and utility.`,
    );
  }
  if (wonGroup != null && lostGroup != null && wonGroup >= lostGroup + 0.7 && wonGroup >= 2) {
    out.good.push(
      `You went in together when it worked (${wonGroup.toFixed(1)} CTs at the bomb on average) vs ${lostGroup.toFixed(1)} in failed retakes.`,
    );
  }
  if (rounds.length === 1 && won.length === 1) {
    const r = won[0];
    out.good.push(
      `Round ${r.round}: defused${r.ctEntryTime != null ? `, first contact ${s(r.ctEntryTime)} after the plant` : ""}.`,
    );
  }
  const uncontested = lost.filter((r) => r.ctEntryTime == null);
  if (uncontested.length && uncontested.every((r) => buyFor(r) === "eco")) {
    out.good.push(
      `Disciplined saves: ${uncontested.length} uncontested plant${uncontested.length === 1 ? "" : "s"} came on eco rounds — keeping guns instead of throwing them away.`,
    );
  }

  // ---- bad patterns ----
  const rushes = lostEntries.filter(
    (r) => (r.ctEntry || []).length === 1 && r.ctEntryTime <= ENTRY_RUSH,
  );
  if (rushes.length) {
    out.bad.push(
      `Rushed solo retakes died for free — ${rushes.length === 1 ? `round ${rushes[0].round} went in at ${s(rushes[0].ctEntryTime)} alone` : `rounds ${list(rushes)} entered solo inside ${ENTRY_RUSH}s`}. Wait for numbers.`,
    );
  }
  if (lostTime != null && wonTime != null && lostTime >= wonTime + 5) {
    out.bad.push(
      `Failed retakes came late (~${s(lostTime)}) once the site was locked down — your defuses came from faster entries (~${s(wonTime)}).`,
    );
  }
  const givenUp = uncontested.filter((r) => buyFor(r) && buyFor(r) !== "eco");
  if (givenUp.length) {
    out.bad.push(
      `${givenUp.length === 1 ? `Round ${givenUp[0].round} was` : `Rounds ${list(givenUp)} were`} never contested — winnable rounds given up without a fight.`,
    );
  }
  const soloAll = lostEntries.length >= 2 && lostEntries.every((r) => (r.ctEntry || []).length === 1);
  if (soloAll && (wonGroup == null || wonGroup < 2)) {
    out.bad.push(
      `Every failed retake was a solo entry — one player at a time into a set-up site never works.`,
    );
  }
  return out;
};

// buildPostPlantNarrative generates the analysis for one player's team.
//   ppRounds — data.rounds from GET /demos/:id/postplant
//   sides    — { [round]: "T" | "CT" } from resolveSides()
//   summaries — the /demos/:id/rounds list (buy types for save context)
export const buildPostPlantNarrative = (ppRounds, sides, summaries) => {
  const buyFor = (r) => {
    const side = sides[r.round];
    if (!side) return null;
    const sum = (summaries || []).find((x) => x.round === r.round);
    return sum ? (side === "CT" ? sum.ctBuy : sum.tBuy) : null;
  };

  const asT = ppRounds.filter((r) => sides[r.round] === "T");
  const asCT = ppRounds.filter((r) => sides[r.round] === "CT");
  const unknown = ppRounds.filter((r) => !sides[r.round]).length;

  return {
    t: tNarrative(asT, buyFor),
    ct: ctNarrative(asCT, buyFor),
    unknown,
  };
};