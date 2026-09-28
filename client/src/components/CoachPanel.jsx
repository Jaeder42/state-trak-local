import React from "react";

// Overlay showing the AI coach's review (from POST /demos/:id/coach).
export const CoachPanel = ({ text, model, onClose }) => (
  <div className="analysis-overlay" onClick={onClose}>
    <div className="analysis-panel coach-panel" onClick={(e) => e.stopPropagation()}>
      <div className="analysis-header">
        <h3>AI coach review{model ? ` · ${model}` : ""}</h3>
        <button className="analysis-close" onClick={onClose} title="Close">
          ×
        </button>
      </div>
      <div className="coach-text">{text}</div>
      <div className="analysis-footer">
        Generated with your own LLM (🔑 settings) from this demo's rounds,
        buys and post-plant analysis.
      </div>
    </div>
  </div>
);