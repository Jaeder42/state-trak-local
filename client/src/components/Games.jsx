import React, { useEffect, useRef, useState } from "react";
import { Frame } from "./Frame.jsx";
import { ScoreBoardPanel } from "./ScoreBoardPanel.jsx";
import { Controls } from "./Controls.jsx";
import { RoundSelector } from "./RoundSelector.jsx";
import { DemoMenu } from "./DemoMenu.jsx";
import { FilterMenu } from "./FilterMenu.jsx";
import { mapDisplayName } from "../maps/config";

const API = "";
const TICK_MS = 16; // ~60fps playback at 1x

export const Games = () => {
  const [demos, setDemos] = useState([]);
  const [demoId, setDemoId] = useState(null);
  const [output, setOutput] = useState({});
  const [index, setIndex] = useState(0);
  const [round, setRound] = useState(0);
  const [loading, setLoading] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [metaData, setMetaData] = useState({});
  const [rounds, setRounds] = useState([]);
  const [focusPlayer, setFocusPlayer] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(null);
  const [toast, setToast] = useState(null);
  const [filters, setFilters] = useState({
    health: true,
    names: true,
    trails: true,
    theater: false,
  });
  const toastTimer = useRef(null);
  const roundFetchId = useRef(0);

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
    const reqId = ++roundFetchId.current;
    try {
      const res = await fetch(`${API}/demos/${id}/${r}`);
      const data = await res.json();
      if (reqId !== roundFetchId.current) return; // a newer round was requested
      setOutput(data);
      setIndex(0);
    } catch (e) {
      console.log(e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchDemos();
  }, []);

  // On demo switch: reset view state, fetch meta + round list.
  useEffect(() => {
    if (!demoId) return;
    setPlaying(false);
    setFocusPlayer(null);
    setLoading(true);
    fetchMeta(demoId);
    fetchRounds(demoId);
    setRound(0);
  }, [demoId]);

  // Clear focus when switching demos (stale steamId otherwise).
  useEffect(() => {
    if (!demoId) setFocusPlayer(null);
  }, [demoId]);

  // Load round data when the selected round changes.
  useEffect(() => {
    if (!demoId) return;
    fetchRound(demoId, round);
  }, [round, demoId]);

  const tick = () => {
    setIndex((i) => {
      const max = (output.frames?.length ?? 1) - 1;
      return i < max ? i + 1 : i;
    });
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps -- tick only closes over output, which is a dep
  useEffect(() => {
    if (!playing) return undefined;
    const inter = setInterval(tick, TICK_MS / speed);
    return () => clearInterval(inter);
  }, [playing, speed, output]);

  // Auto-advance to the next round (or stop after the last one).
  useEffect(() => {
    if (!playing) return;
    const max = (output.frames?.length ?? 1) - 1;
    if (index < max) return;
    if (output.round != null && output.round !== round) return; // next round still loading
    if (round < rounds.length - 1) {
      setRound((r) => r + 1);
    } else {
      setPlaying(false);
    }
  }, [index, playing, round, rounds.length, output]);

  const tooglePlay = (play) => {
    setPlaying(play);
  };
  const selectPlayer = (steamId) => {
    setFocusPlayer((prev) => (prev === steamId ? null : steamId));
  };
  const toggleFilter = (key) => {
    setFilters((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const showToast = (msg) => {
    setToast(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 5000);
  };

  const deleteDemo = async (id) => {
    try {
      await fetch(`${API}/demos/${id}`, { method: "DELETE" });
      if (demoId === id) {
        setDemoId(null);
        setOutput({});
        setRounds([]);
        setMetaData({});
      }
      await fetchDemos();
    } catch (err) {
      console.error(err);
    }
  };

  // Upload via XHR so we get real upload progress, then poll parse progress.
  const onUpload = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = ""; // allow re-selecting the same file
    setUploading(true);
    setUploadProgress({ phase: "uploading", pct: 0 });
    const form = new FormData();
    form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${API}/upload`);
    xhr.upload.onprogress = (ev) => {
      if (ev.lengthComputable) {
        setUploadProgress({
          phase: "uploading",
          pct: Math.round((ev.loaded / ev.total) * 100),
        });
      }
    };
    xhr.onload = () => {
      try {
        const { id } = JSON.parse(xhr.responseText);
        setUploadProgress({ phase: "parsing", pct: 0 });
        pollStatus(id);
      } catch (err) {
        console.error(err);
        setUploading(false);
        setUploadProgress(null);
        showToast("Upload failed: " + (xhr.responseText || "unknown error"));
      }
    };
    xhr.onerror = () => {
      setUploading(false);
      setUploadProgress(null);
      showToast("Upload failed");
    };
    xhr.send(form);
  };

  const pollStatus = (id) => {
    const poll = async () => {
      try {
        const res = await fetch(`${API}/demos/${id}/status`);
        const status = await res.json();
        if (status.status === "done") {
          setUploading(false);
          setUploadProgress(null);
          await fetchDemos();
          setDemoId(id);
        } else if (status.status === "error") {
          setUploading(false);
          setUploadProgress(null);
          showToast("Failed to parse demo: " + (status.error || "unknown error"));
        } else {
          if (typeof status.progress === "number" && status.progress > 0) {
            setUploadProgress({ phase: "parsing", pct: status.progress });
          }
          setTimeout(poll, 1000);
        }
      } catch (err) {
        console.error(err);
        setUploading(false);
        setUploadProgress(null);
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
        setIndex((i) => Math.min(i + 1, (output.frames?.length ?? 1) - 1));
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        setPlaying(false);
        setIndex((i) => Math.max(i - 1, 0));
      } else if (e.key === "Escape") {
        setFocusPlayer(null);
      } else if (e.key === "," || e.key === "[") {
        e.preventDefault();
        if (round > 0) {
          setPlaying(false);
          setRound((r) => r - 1);
        }
      } else if (e.key === "." || e.key === "]") {
        e.preventDefault();
        if (round < rounds.length - 1) {
          setPlaying(false);
          setRound((r) => r + 1);
        }
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [output.frames?.length, round, rounds.length]);

  // Throttle scoreboard updates to ~5Hz; the tables don't need 60fps.
  const sbIndex = output.frames
    ? Math.min(Math.floor(index / 12) * 12, output.frames.length - 1)
    : 0;
  const sbFrame = output.frames?.[sbIndex];

  return (
    <div>
      <DemoMenu
        demos={demos}
        demoId={demoId}
        uploading={uploading}
        uploadProgress={uploadProgress}
        onSelectDemo={setDemoId}
        onUpload={onUpload}
        onDeleteDemo={deleteDemo}
      />

      {toast && <div className="toast">{toast}</div>}

      {!demoId ? (
        <div className="empty-state">Upload or select a demo to begin</div>
      ) : loading ? (
        <div className="empty-state">…loading</div>
      ) : (
        <>
          <h1>{mapDisplayName(metaData.map)}</h1>
          <div className={`main-layout ${filters.theater ? "theater" : ""}`}>
            <ScoreBoardPanel
              frame={sbFrame}
              onSelectPlayer={selectPlayer}
              focusPlayer={focusPlayer}
            />
            <Frame
              mapName={metaData.map}
              frame={
                output.frames
                  ? output.frames[Math.min(index, output.frames.length - 1)]
                  : null
              }
              frames={output.frames}
              kills={output.kills}
              index={index}
              focusPlayer={focusPlayer}
              filters={filters}
              onSelectPlayer={selectPlayer}
            />
          </div>
          <Controls
            playing={playing}
            onTogglePlay={tooglePlay}
            speed={speed}
            onSpeedChange={setSpeed}
          >
            <FilterMenu filters={filters} onToggle={toggleFilter} />
          </Controls>

          <RoundSelector
            rounds={rounds}
            currentRound={round}
            onSelect={(i) => {
              setPlaying(false);
              setRound(i);
            }}
            output={output}
            index={index}
            onIndexChange={setIndex}
          />
        </>
      )}
    </div>
  );
};