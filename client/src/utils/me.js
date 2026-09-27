// "My" player identity — the demo owner's Steam ID, used to orient the UI
// around the team they played on (scoreboard order, radar emphasis, kill feed
// tints, round win/loss colors). Stored in localStorage so it survives
// reloads; the default is this app's owner (it's a personal local tool —
// change it in the settings panel or here).
const STORAGE_KEY = "mySteamId";
const DEFAULT_STEAM_ID = "REDACTED_STEAM_ID";

export const getMySteamId = () =>
  localStorage.getItem(STORAGE_KEY) ?? DEFAULT_STEAM_ID;

export const setMySteamId = (id) => {
  const value = String(id).trim();
  if (value) {
    localStorage.setItem(STORAGE_KEY, value);
  } else {
    localStorage.removeItem(STORAGE_KEY); // fall back to the default
  }
};