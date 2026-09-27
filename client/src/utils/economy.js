// Buy type metadata for economy badges.
// label/short are the texts used in pills (scoreboard) and chips (round
// selector buttons), color is the badge background, desc feeds tooltips.
export const BUY_TYPES = {
  pistol: {
    label: "PISTOL",
    short: "P",
    color: "#8d99a6",
    desc: "pistol round",
  },
  eco: {
    label: "ECO",
    short: "ECO",
    color: "#e05d5d",
    desc: "eco — saving for a future buy",
  },
  hero: {
    label: "HERO",
    short: "HERO",
    color: "#a876f2",
    desc: "hero buy — one player forces, the rest save",
  },
  force: {
    label: "FORCE",
    short: "FRC",
    color: "#ff8c42",
    desc: "force buy — cheap weapons on a small bank",
  },
  half: {
    label: "HALF",
    short: "HALF",
    color: "#f2c14e",
    desc: "half buy — some rifles, some saves",
  },
  kept: {
    label: "KEPT",
    short: "KEPT",
    color: "#4ec9b0",
    desc: "kept — weapons carried over from the last round",
  },
  full: {
    label: "FULL",
    short: "FULL",
    color: "#4caf50",
    desc: "full buy — rifles all around",
  },
};

export const buyType = (type) => (type ? BUY_TYPES[type] || null : null);