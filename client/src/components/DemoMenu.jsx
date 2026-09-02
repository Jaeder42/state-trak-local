import React, { useRef, useState } from "react";

export const DemoMenu = ({
  demos,
  demoId,
  uploading,
  onSelectDemo,
  onUpload,
  onDeleteDemo,
}) => {
  const [open, setOpen] = useState(false);
  const fileInputRef = useRef(null);

  return (
    <div className="demo-menu">
      <button
        className="demo-menu-toggle"
        onClick={() => setOpen((o) => !o)}
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
            {demos.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name} {d.status === "parsing" ? "(parsing…)" : ""}
              </option>
            ))}
          </select>
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            {uploading ? "Uploading…" : "Upload .dem"}
          </button>
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
