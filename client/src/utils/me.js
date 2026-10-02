// "My" player identity — the Steam ID of whoever is using the app, used to
// orient the UI around the team they played on (scoreboard order, radar
// emphasis, kill feed tints, round win/loss colors). Stored in localStorage
// so it survives reloads; empty until set in the settings panel (⚙). With
// no ID set, the viewer stays in neutral coloring.
const STORAGE_KEY = "mySteamId";

export const getMySteamId = () => localStorage.getItem(STORAGE_KEY) ?? "";

export const setMySteamId = (id) => {
  const value = String(id).trim();
  if (value) {
    localStorage.setItem(STORAGE_KEY, value);
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
};