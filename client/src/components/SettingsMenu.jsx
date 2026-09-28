import React, { useState } from "react";
import { getSetting, setSetting } from "../utils/settings";

// 🔑 bring-your-own keys: TypeSafe (JEV) + any OpenAI-compatible LLM.
// Values live in this browser's localStorage and are sent per request.
export const SettingsMenu = () => {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState(() => ({
    jevKey: getSetting("jevKey"),
    llmBaseUrl: getSetting("llmBaseUrl"),
    llmModel: getSetting("llmModel"),
    llmKey: getSetting("llmKey"),
  }));

  const change = (key) => (e) => {
    const value = e.target.value;
    setValues((prev) => ({ ...prev, [key]: value }));
    setSetting(key, value);
  };

  const field = (key, label, placeholder, type = "text") => (
    <label className="settings-field">
      <span className="settings-label">{label}</span>
      <input
        className="settings-input"
        type={type}
        value={values[key]}
        onChange={change(key)}
        placeholder={placeholder}
        spellCheck={false}
      />
    </label>
  );

  return (
    <div className="filter-menu">
      <button
        className="filter-toggle"
        onClick={() => setOpen((o) => !o)}
        title="Keys & AI settings"
      >
        🔑
      </button>
      {open && (
        <div className="filter-panel settings-panel">
          <div className="settings-section">TypeSafe — JEV analysis</div>
          {field(
            "jevKey",
            "API key",
            "TypeSafe key (console.typesafe.ai)",
            "password",
          )}
          <div className="settings-section">LLM — AI coach</div>
          {field(
            "llmBaseUrl",
            "Base URL",
            "https://api.openai.com/v1 (or http://localhost:11434/v1 for Ollama)",
          )}
          {field("llmModel", "Model", "gpt-4o-mini")}
          {field("llmKey", "API key", "optional for local models", "password")}
          <span className="settings-note">
            Stored in this browser only — sent per request, never saved
            server-side.
          </span>
        </div>
      )}
    </div>
  );
};