// Bring-your-own keys & LLM settings, stored in this browser's localStorage.
// Sent to the server per request — never persisted server-side, so each
// user of a shared deployment uses (and pays for) their own keys.

const STORAGE = {
  jevKey: "statetrak_jev_key",
  llmBaseUrl: "statetrak_llm_base_url",
  llmModel: "statetrak_llm_model",
  llmKey: "statetrak_llm_key",
};

export const getSetting = (key) =>
  localStorage.getItem(STORAGE[key]) ?? "";

export const setSetting = (key, value) => {
  const v = String(value).trim();
  if (v) {
    localStorage.setItem(STORAGE[key], v);
  } else {
    localStorage.removeItem(STORAGE[key]);
  }
};

export const getLLMConfig = () => ({
  baseUrl: getSetting("llmBaseUrl"),
  model: getSetting("llmModel"),
  apiKey: getSetting("llmKey"),
});

export const llmConfigured = () => {
  const { baseUrl, model } = getLLMConfig();
  return !!(baseUrl && model);
};