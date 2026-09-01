import React, { useEffect, useState, useRef } from "react";
import { Frame } from "./Frame.jsx";
import { ScoreBoardPanel } from "./ScoreBoardPanel.jsx";
import { Controls } from "./Controls.jsx";
import { RoundSelector } from "./RoundSelector.jsx";

const API = "";

export const Games = () => {
  const [demos, setDemos] = useState([]);
  const [demoId, setDemoId] = useState(null);
  const [output, setOutput] = useState({});
  const [index, setIndex] = useState(0);
  const [round, setRound] = useState(0);
  const [loading, setLoading] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [intervalID, setIntervalID] = useState(null);
  const [metaData, setMetaData] = useState({});
  const [rounds, setRounds] = useState([]);
  const [focusPlayer, setFocusPlayer] = useState(null);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef(null);

  const fetchDemos = async () => {
    try {
      const res = await fetch(`${API}/demos`);
      const json = await res.json();
      setDemos(Array.isArray(json) ? json : []);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchMeta = async (id) => {
    try {
      const meta = await fetch(`${API}/demos/${id}/output`);
      const json = await meta.json();
      setMetaData(json);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchRounds = async (id) => {
    try {
      const res = await fetch(`${API}/demos/${id}/rounds`);
      setRounds(await res.json());
    } catch (err) {
      console.error(err);
    }
  };

  const fetchRound = async (id, r) => {
    try {
      const roundData = await fetch(`${API}/demos/${id}/${r}`);
      setOutput(await roundData.json());
      setIndex(0);
      setPlaying(false);
    } catch (e) {
      console.log(e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchDemos();
  }, []);

  useEffect(() => {
    if (!demoId) return;
    setLoading(true);
    fetchMeta(demoId);
    fetchRounds(demoId);
    setRound(0);
  }, [demoId]);

  useEffect(() => {
    if (!demoId) return;
    fetchRound(demoId, round);
  }, [round, demoId]);

  useEffect(() => {
    if (playing) {
      const inter = setInterval(() => tick(), 16);
      setIntervalID(inter);
    } else {
      clearInterval(intervalID);
    }
  }, [playing]);

  const tick = () => {
    if (index < output.frames?.length - 1) {
      setIndex((index) => +index + 1);
    }
  };

  const tooglePlay = (play) => {
    setPlaying(play);
  };
  const selectPlayer = (steamId) => {
    setFocusPlayer((prev) => (prev === steamId ? null : steamId));
  };

  const onUpload = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    const form = new FormData();
    form.append("file", file);
    try {
      const res = await fetch(`${API}/upload`, { method: "POST", body: form });
      const { id } = await res.json();
      pollStatus(id);
    } catch (err) {
      console.error(err);
      setUploading(false);
    }
  };

  const pollStatus = (id) => {
    const poll = async () => {
      try {
        const res = await fetch(`${API}/demos/${id}/status`);
        const status = await res.json();
        if (status.status === "done") {
          setUploading(false);
          await fetchDemos();
          setDemoId(id);
        } else if (status.status === "error") {
          setUploading(false);
          alert("Failed to parse demo: " + status.error);
        } else {
          setTimeout(poll, 1000);
        }
      } catch (err) {
        console.error(err);
        setUploading(false);
      }
    };
    poll();
  };

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.target.tagName === "INPUT") return;
      if (e.code === "Space") {
        e.preventDefault();
        setPlaying((p) => !p);
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        setPlaying(false);
        setIndex((i) => Math.min(i + 1, output.frames?.length - 1 ?? 0));
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        setPlaying(false);
        setIndex((i) => Math.max(i - 1, 0));
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [output.frames?.length]);

  return (
    <div>
      <div className="demo-bar">
        <select
          value={demoId || ""}
          onChange={(e) => setDemoId(e.target.value)}
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
        <button onClick={() => fileInputRef.current?.click()} disabled={uploading}>
          {uploading ? "Uploading…" : "Upload .dem"}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept=".dem"
          style={{ display: "none" }}
          onChange={onUpload}
        />
      </div>

      {!demoId ? (
        <div className="empty-state">Upload or select a demo to begin</div>
      ) : loading ? (
        <div>...loading</div>
      ) : (
        <>
          <h1>{metaData.map}</h1>
          <div className="main-layout">
            <ScoreBoardPanel
              frame={output.frames?.[index]}
              onSelectPlayer={selectPlayer}
              focusPlayer={focusPlayer}
            />
            <Frame
              mapName={metaData.map}
              frame={output.frames
                ? output.frames[Math.min(index, output.frames?.length - 1)]
                : null}
              focusPlayer={focusPlayer}
            />
          </div>
          <Controls playing={playing} onTogglePlay={tooglePlay} />

          <RoundSelector
            rounds={rounds}
            currentRound={round}
            onSelect={setRound}
            index={index}
            max={output.frames?.length - 1 ?? 0}
            onIndexChange={setIndex}
          />
        </>
      )}
    </div>
  );
};
