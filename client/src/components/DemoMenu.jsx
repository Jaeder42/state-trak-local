import React, { useEffect, useRef, useState } from "react";
import { prettyDemoName } from "../utils/demoName";

// Upload/parse progress readout — shown inside the menu and, while active,
// as a floating chip that survives the menu being closed (Games renders
// the float itself with the same component).
export const UploadProgress = ({ progress }) =>
  progress ? (
    <div className="upload-progress">
      <div className="upload-progress-label">
        {progress.phase === "uploading" ? "Uploading" : "Parsing"}{" "}
        {progress.pct}%
      </div>
      <div className="upload-progress-track">
        <div
          className="upload-progress-fill"
          style={{ width: `${progress.pct}%` }}
        />
      </div>
    </div>
  ) : null;

export const DemoMenu = ({
  demos,
  demoId,
  uploading,
  uploadProgress,
  analysisRunning,
  postplantRunning,
  coachRunning,
  onSelectDemo,
  onUpload,
  onDeleteDemo,
  onRunAnalysis,
  onRunPostPlant,
  onRunCoach,
}) => {
  const [open, setOpen] = useState(false);
  const fileInputRef = useRef(null);

  // Close on Escape — panels above this menu handle their own layering in
  // Games' keydown; menus just dismiss themselves.
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <div className="demo-menu">
      <button
        className="demo-menu-toggle"
        onClick={() => setOpen((o) => !o)}
        aria-label="Demos menu"
        title="Demos"
      >
        ☰
      </button>
      {open && (
        <div className="demo-menu-panel">
          <select
            value={demoId || ""}
            onChange={(e) => {
              onSelectDemo(e.target.value);
              setOpen(false);
            }}
          >
            <option value="" disabled>
              Select a demo
            </option>
            {demos.map((d) => {
              const pretty = prettyDemoName(d.name);
              return (
                <option key={d.id} value={d.id} title={d.name}>
                  {pretty.title}
                  {pretty.sub ? ` · ${pretty.sub}` : ""}{" "}
                  {d.status === "parsing" ? "(parsing…)" : ""}
                </option>
              );
            })}
          </select>
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            {uploading ? "Uploading…" : "Upload .dem"}
          </button>
          {uploadProgress && <UploadProgress progress={uploadProgress} />}
          {demoId && (
            <button
              onClick={() => onRunCoach(demoId)}
              disabled={coachRunning}
            >
              {coachRunning ? "Coach thinking…" : "AI coach (your LLM)"}
            </button>
          )}
          {demoId && (
            <button
              onClick={() => onRunPostPlant(demoId)}
              disabled={postplantRunning}
            >
              {postplantRunning ? "Analyzing…" : "Post-plant analysis"}
            </button>
          )}
          {demoId && (
            <button
              onClick={() => onRunAnalysis(demoId)}
              disabled={analysisRunning}
            >
              {analysisRunning ? "Analyzing…" : "JEV analysis"}
            </button>
          )}
          {demoId && (
            <button
              className="delete-demo"
              onClick={() => {
                if (window.confirm("Delete this demo?")) {
                  onDeleteDemo(demoId);
                  setOpen(false);
                }
              }}
            >
              Delete demo
            </button>
          )}
          <input
            ref={fileInputRef}
            type="file"
            accept=".dem"
            style={{ display: "none" }}
            onChange={onUpload}
          />
        </div>
      )}
    </div>
  );
};