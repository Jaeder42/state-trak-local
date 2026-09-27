import React from "react";

const pct = (v) => (typeof v === "number" ? v.toFixed(2) : "–");

const TeamCell = ({ t }) => {
  if (!t) return <td>–</td>;
  const title = t.jev
    ? `baseline: ${t.baseline} · jev: ${t.jev} (confidence ${t.confidence.toFixed(2)})`
    : `baseline: ${t.baseline}`;
  return (
    <td className={t.agree ? "" : "disagree"} title={title}>
      {t.baseline}
      {t.jev && (
        <>
          {" → "}
          {t.jev}
          {!t.agree && " ⚠"}
        </>
      )}
    </td>
  );
};

export const AnalysisPanel = ({ analysis, onClose }) => {
  if (!analysis) return null;
  const rounds = analysis.rounds || [];
  return (
    <div className="analysis-overlay" onClick={onClose}>
      <div className="analysis-panel" onClick={(e) => e.stopPropagation()}>
        <div className="analysis-header">
          <h3>
            JEV round analysis
            {analysis.model ? ` · ${analysis.model}` : ""}
          </h3>
          <button className="analysis-close" onClick={onClose} title="Close">
            ×
          </button>
        </div>
        <p className="analysis-sub">
          baseline → JEV per team; ⚠ marks rounds where JEV disagrees with the
          threshold classifier. Eco columns are JEV's probability the team
          played an eco.
        </p>
        <table className="analysis-table">
          <thead>
            <tr>
              <th>R#</th>
              <th>Winner</th>
              <th>CT buy</th>
              <th>T buy</th>
              <th>CT eco</th>
              <th>T eco</th>
            </tr>
          </thead>
          <tbody>
            {rounds.map((r) => (
              <tr key={r.round}>
                <td>{r.round}</td>
                <td>{r.winner}</td>
                <TeamCell t={r.ct} />
                <TeamCell t={r.t} />
                <td>{r.ct ? pct(r.ct.eco) : "–"}</td>
                <td>{r.t ? pct(r.t.eco) : "–"}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {analysis.usage && (
          <div className="analysis-footer">
            {analysis.usage.input_tokens} in / {analysis.usage.output_tokens}{" "}
            out tokens · cached ·{" "}
            {analysis.generatedAt
              ? new Date(analysis.generatedAt).toLocaleString()
              : ""}
          </div>
        )}
      </div>
    </div>
  );
};