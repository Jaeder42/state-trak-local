// Humanize the raw demo filenames the backend stores, e.g.
//   "2025-11-03_20-39-21_-1_de_ancient_team_Cayeen_vs_team_HunkDaddy.dem"
//   -> { title: "Cayeen vs HunkDaddy", sub: "Ancient · 2025-11-03 20:39" }
// Falls back to the cleaned filename when the pattern doesn't match (the
// raw name is always kept for tooltips/ownership).

const DATE_TIME = /^(\d{4}-\d{2}-\d{2})_(\d{2})-(\d{2})-(\d{2})/;
const MAP_TOKEN = /\b(de_[a-z0-9]+)\b/;
const TEAMS = /team_(.+?)_vs_team_(.+)$/;

const mapLabel = (token) => {
  if (!token) return null;
  return token
    .replace(/^de_/, "")
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
};

export const prettyDemoName = (raw) => {
  if (!raw) return { title: "Untitled demo", sub: "" };
  const cleaned = raw.replace(/\.dem$/i, "");
  const parts = [];
  const when = cleaned.match(DATE_TIME);
  if (when) parts.push(`${when[1]} ${when[2]}:${when[3]}`);
  const mapToken = cleaned.match(MAP_TOKEN);
  const map = mapLabel(mapToken?.[1]);
  const teams = cleaned.match(TEAMS);
  const title = teams ? `${teams[1]} vs ${teams[2]}` : cleaned.replace(/_/g, " ");
  const sub = [map, ...parts].filter(Boolean).join(" · ");
  return { title, sub };
};